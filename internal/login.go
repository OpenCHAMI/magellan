package magellan

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pkg/browser"
)

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
