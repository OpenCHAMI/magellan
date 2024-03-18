package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/OpenCHAMI/magellan/internal/log"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	loginUrl   string
	refreshUrl string
	targetHost string
	targetPort int
	tokenPath  string
	forceLogin bool
	noBrowser  bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in with identity provider for access token",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		// make application logger
		l := log.NewLogger(logrus.New(), logrus.DebugLevel)

		// check if we have a valid JWT before starting login
		if !forceLogin {
			// try getting the access token from env var
			testToken, err := LoadAccessToken()
			if err != nil {
				l.Log.Errorf("failed to load access token: %v", err)
			}

			// parse into jwt.Token to validate
			token, err := jwt.Parse([]byte(testToken))
			if err != nil {
				fmt.Printf("failed to parse access token contents: %v\n", err)
				return
			}
			// check if the token is invalid and we need a new one
			err = jwt.Validate(token)
			if err != nil {
				fmt.Printf("failed to validate access token...fetching a new one")

				// try to get access token with refresh token if it's valid
				bearer, err := magellan.Refresh(refreshUrl, targetHost, targetPort)
				if err != nil {
					return
				} else {
					fmt.Printf("successfully fetched new access token...skipping login(use the '-f/--force' flag to login anyway)\n")

					// if we got a new token successfully, save it to the token path
					if bearer.AccessToken != "" && tokenPath != "" {
						err := os.WriteFile(tokenPath, []byte(bearer.AccessToken), os.ModePerm)
						if err != nil {
							fmt.Printf("failed to write access token to file: %v\n", err)
						}
					}
					return
				}

			} else {
				fmt.Printf("found a valid token...skipping login (use the '-f/--force' flag to login anyway)")
				return
			}
		}

		// start the login flow
		var err error
		bearer, err := magellan.Login(loginUrl, targetHost, targetPort)
		if errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("\n=========================================\nServer closed.\n=========================================\n\n")
		} else if err != nil {
			fmt.Printf("failed to start server: %v\n", err)
		}

		// if we got a new token successfully, save it to the token path
		if bearer.AccessToken != "" && tokenPath != "" {
			err := os.WriteFile(tokenPath, []byte(bearer.AccessToken), os.ModePerm)
			if err != nil {
				fmt.Printf("failed to write access token to file: %v\n", err)
			}
		}
	},
}

func init() {
	loginCmd.Flags().StringVar(&loginUrl, "login-url", "http://127.0.0.1:3333/login", "set the login URL")
	loginCmd.Flags().StringVar(&refreshUrl, "refresh-url", "http://127.0.0.1:3333/refresh", "set the refresh URL")
	loginCmd.Flags().StringVar(&targetHost, "target-host", "127.0.0.1", "set the target host to return the access code")
	loginCmd.Flags().IntVar(&targetPort, "target-port", 5000, "set the target host to return the access code")
	loginCmd.Flags().BoolVarP(&forceLogin, "force", "f", false, "start the login process even with a valid token")
	loginCmd.Flags().StringVar(&tokenPath, "token-path", ".ochami-token", "set the path the load/save the access token")
	loginCmd.Flags().BoolVar(&noBrowser, "no-browser", false, "prevent the default browser from being opened automatically")
	rootCmd.AddCommand(loginCmd)
}
