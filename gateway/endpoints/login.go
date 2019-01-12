package endpoints

import (
	"encoding/json"
	"net/http"
	"path"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/securecookie"
	"github.com/sahib/config"
)

var cookieHandler = securecookie.New(
	securecookie.GenerateRandomKey(64),
	securecookie.GenerateRandomKey(32),
)

func getUserName(r *http.Request) string {
	cookie, err := r.Cookie("session")
	if err != nil {
		return ""
	}

	cookieValue := make(map[string]string)
	if err := cookieHandler.Decode("session", cookie.Value, &cookieValue); err != nil {
		log.Debugf("failed to decode cookie: %v", err)
		return ""
	}

	return cookieValue["name"]
}

func setSession(userName string, w http.ResponseWriter, r *http.Request) {
	value := map[string]string{
		"name": userName,
	}

	encoded, err := cookieHandler.Encode("session", value)
	if err != nil {
		log.Warningf("failed to set cookie: %v", err)
		return
	}

	// TODO: Set cookie domain?
	http.SetCookie(w, &http.Cookie{
		Name:  "session",
		Value: encoded,
		Path:  "/",
	})
}

func clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   "session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

// validateUserForPath checks if `r` is allowed to view `nodePath`.
func validateUserForPath(cfg *config.Config, nodePath string, r *http.Request) bool {
	if getUserName(r) == "" {
		return false
	}

	// build a map for constant lookup time
	folders := make(map[string]bool)
	for _, folder := range cfg.Strings("folders") {
		folders[folder] = true
	}

	if !strings.HasPrefix(nodePath, "/") {
		nodePath = "/" + nodePath
	}

	curr := nodePath
	for curr != "" {
		if folders[curr] {
			return true
		}

		next := path.Dir(curr)
		if curr == "/" && next == curr {
			// We've gone up too much:
			break
		}

		curr = next
	}

	// No fitting path found:
	return false
}

///////

type LoginHandler struct {
	State
}

func NewLoginHandler(s State) *LoginHandler {
	return &LoginHandler{State: s}
}

type LoginRequest struct {
	Username string `json="username"`
	Password string `json="password"`
}

func (lih *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	loginReq := &LoginRequest{}
	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		jsonifyErrf(w, http.StatusBadRequest, "bad json")
		return
	}

	if loginReq.Username == "" || loginReq.Password == "" {
		jsonifyErrf(w, http.StatusBadRequest, "empty password or username")
		return
	}

	cfgUser := lih.cfg.String("auth.user")
	cfgPass := lih.cfg.String("auth.pass")

	if cfgUser != loginReq.Username || cfgPass != loginReq.Password {
		jsonifyErrf(w, http.StatusForbidden, "bad credentials")
		return
	}

	setSession(cfgUser, w, r)
	jsonifySuccess(w)
}

///////

type LogoutHandler struct {
	State
}

func NewLogoutHandler(s State) *LogoutHandler {
	return &LogoutHandler{State: s}
}

func (loh *LogoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user := getUserName(r)
	if user == "" {
		jsonifyErrf(w, http.StatusBadRequest, "not logged in")
		return
	}

	clearSession(w)
	jsonifySuccess(w)
}

///////

type WhoamiHandler struct {
	State
}

func NewWhoamiHandler(s State) *WhoamiHandler {
	return &WhoamiHandler{State: s}
}

type WhoamiResponse struct {
	IsLoggedIn bool   `json:"is_logged_in"`
	User       string `json:"user"`
}

func (wh *WhoamiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user := getUserName(r)
	if user != "" {
		setSession(user, w, r)
	}

	jsonify(w, http.StatusOK, WhoamiResponse{
		IsLoggedIn: len(user) > 0,
		User:       user,
	})
}