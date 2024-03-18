package magellan

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pkg/browser"
)

type BearerToken struct {
	AccessToken string
	IdToken     string
	ExpiresAt   string
	TokenType   string
}

func Login(loginUrl string, targetHost string, targetPort int) (*BearerToken, error) {
	var token *BearerToken

	// check and make sure the login URL isn't empty
	if loginUrl == "" {
		return nil, fmt.Errorf("no login URL provided")
	}

	// if a target host and port are provided, then add to URL
	if targetHost != "" && targetPort > 0 && targetPort < 65536 {
		loginUrl += fmt.Sprintf("?target=http://%s:%d", targetHost, targetPort)
	}

	// open browser with the specified URL
	err := browser.OpenURL(loginUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to open browser: %v", err)
	}

	// start a temporary server to listen for token
	s := http.Server{
		Addr: fmt.Sprintf("%s:%d", targetHost, targetPort),
	}
	r := chi.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// try and extract access token from headers
		token.AccessToken = r.Header.Get("access_token")
		bearer := r.Header.Get("bearer")
		if bearer != "" {
			json.Unmarshal([]byte(bearer), &token)
		}
		s.Close()
	})
	return token, s.ListenAndServe()
}

func Refresh(refreshUrl string, targetHost string, targetPort int) (*BearerToken, error) {
	var token *BearerToken

	// check and make sure the refresh URL isn't empty
	if refreshUrl == "" {
		return nil, fmt.Errorf("no refresh URL provided")
	}

	// if a target host and port are provided, then add to URL
	if targetHost != "" && targetPort > 0 && targetPort < 65536 {
		refreshUrl += fmt.Sprintf("?target=http://%s:%d", targetHost, targetPort)
	}

	// start a temporary server to listen for token
	s := http.Server{
		Addr: fmt.Sprintf("%s:%d", targetHost, targetPort),
	}
	r := chi.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// try and extract access token from headers
		token.AccessToken = r.Header.Get("access_token")
		bearer := r.Header.Get("bearer")
		if bearer != "" {
			json.Unmarshal([]byte(bearer), &token)
		}
		s.Close()
	})
	return token, s.ListenAndServe()
}
