package magellan

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pkg/browser"
)

// Login() initiates the process to retrieve an access token from an identity provider.
// This function is especially designed to work by OPAAL, but will propably be changed
// in the future to be more agnostic.
//
// The 'targetHost' and 'targetPort' parameters should point to the target host/port
// to create a temporary server to receive the access token. If an empty 'targetHost'
// or an invalid port range is passed, then neither of the parameters will be used
// and no server will be started.
//
// Returns an access token as a string if successful and nil error. Otherwise, returns
// an empty string with an error set.
func Login(loginUrl string, targetHost string, targetPort int) (string, error) {
	var accessToken string

	// check and make sure the login URL isn't empty
	if loginUrl == "" {
		return "", fmt.Errorf("no login URL provided")
	}

	// if a target host and port are provided, then add to URL
	if targetHost != "" && targetPort > 0 && targetPort < 65536 {
		loginUrl += fmt.Sprintf("?target=http://%s:%d", targetHost, targetPort)
	}

	// open browser with the specified URL
	err := browser.OpenURL(loginUrl)
	if err != nil {
		return "", fmt.Errorf("failed to open browser: %v", err)
	}

	// start a temporary server to listen for token
	s := http.Server{
		Addr: fmt.Sprintf("%s:%d", targetHost, targetPort),
	}
	r := chi.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// try and extract access token from headers
		accessToken = r.Header.Get("access_token")
		s.Close()
	})
	return accessToken, s.ListenAndServe()
}
