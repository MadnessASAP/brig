package testlayer

import (
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/disorganizer/brig/repo"
	"github.com/disorganizer/brig/transfer"
	"github.com/disorganizer/brig/transfer/moose"
	"github.com/disorganizer/brig/util/testwith"
)

func WithConnector(t *testing.T, user string, fc func(c *transfer.Connector)) {
	testwith.WithRepo(t, user, user+"pass", func(rp *repo.Repository) {
		if err := rp.IPFS.Online(); err != nil {
			t.Errorf("Cannot go online with IPFS repo: %v", err)
			return
		}

		lay := moose.NewLayer(rp.IPFS, context.Background())
		con := transfer.NewConnector(lay, rp)

		if err := con.Connect(); err != nil {
			t.Errorf("Cannot connect: %v", err)
			return
		}

		t.Logf("Entering test for %s's connector", user)
		fc(con)
		t.Logf("Leaving test for %s's connector", user)

		if err := con.Disconnect(); err != nil {
			t.Errorf("Cannot disconnect: %v", err)
			return
		}
	})
}

// type ManyConnectorsFunc func(cs []*transfer.Connector)
//
//
// func withManyConnectors(t *testing.T, users []string, cs []*transfer.Connector, f ManyConnectorsFunc) {
// 	if len(users) == 0 {
// 		f(cs)
// 		return
// 	}
//
// 	WithConnector(t, users[0], func(c *transfer.Connector) {
// 		withManyConnectors(t, users[1:], append(cs, c), f)
// 	})
// }
//
// func WithManyConnectors(t *testing.T, users []string, f ManyConnectorsFunc) {
// 	withManyConnectors(t, users, []*transfer.Connector{}, f)
// }

func WithManyConnectors(t *testing.T, users []string, f func(cs []*transfer.Connector)) {
	// You probably could re-use the same waitgroup, but let's give it clear names:
	cleanupWg := sync.WaitGroup{}
	cleanupWg.Add(len(users))
	waitWg := sync.WaitGroup{}
	waitWg.Add(1)
	setupWg := sync.WaitGroup{}
	setupWg.Add(len(users))

	cns := []*transfer.Connector{}
	mu := sync.Mutex{}

	// Trigger setup of connectors in parallel:
	for _, user := range users {
		go func(user string) {
			WithConnector(t, user, func(cn *transfer.Connector) {
				// Append it to the connector list:
				mu.Lock()
				cns = append(cns, cn)
				mu.Unlock()

				// Count down one setup'd connector:
				setupWg.Done()

				// Wait for testcase to finish:
				waitWg.Wait()
			})

			// Report that 1 connector was cleaned up.
			cleanupWg.Done()
		}(user)
	}

	// Wait for all connectors to show up:
	setupWg.Wait()

	// Sometimes ipfs does not seem to be fully online yet,
	// wait a short bit therefore.
	time.Sleep(2 * time.Second)

	// Call testcase:
	f(cns)

	// Notify go routines that the testcase finished
	// and we may cleanup the connectors again:
	waitWg.Done()

	// Wait until the cleanup is finished:
	cleanupWg.Wait()
}

func TestConversation(t *testing.T) {
	WithManyConnectors(t, []string{"alice", "bob"}, func(cs []*transfer.Connector) {
		ac, bc := cs[0], cs[1]

		br, ar := bc.Repo(), ac.Repo()
		berr := br.Remotes.Insert(repo.NewRemoteFromPeer(ar.Peer()))
		if berr != nil {
			t.Errorf("Bob has no friends: %v", berr)
			return
		}

		aerr := ar.Remotes.Insert(repo.NewRemoteFromPeer(br.Peer()))
		if aerr != nil {
			t.Errorf("Alice has no friends: %v", aerr)
			return
		}

		apc, err := ac.Dial(br.Peer())
		if err != nil {
			t.Errorf("Alice cannot dial to bob: %v", err)
			return
		}

		// Spam in some queries:
		for i := 0; i < 10; i++ {
			v, err := apc.QueryStoreVersion()
			if err != nil {
				t.Errorf("Usage of api client failed: %v", err)
				return
			}

			if v <= 0 {
				t.Errorf("Version should be any positive number")
				return
			}
		}

		if err := apc.Close(); err != nil {
			t.Errorf("Alice cannot close apiclient to bob: %v", err)
			return
		}
	})
}