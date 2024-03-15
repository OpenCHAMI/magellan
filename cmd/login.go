package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/spf13/cobra"
)

var (
	loginUrl   string
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
		// check if we have a valid JWT before starting login
		if !forceLogin {
			// try getting the access token from env var
			testToken := []byte(os.Getenv("OCHAMI_ACCESS_TOKEN"))
			if testToken == nil {
				// try reading access token from a file
				b, err := os.ReadFile(tokenPath)
				if err != nil {
					fmt.Printf("failed to read access token from file: %v\n", err)
					return
				}
				testToken = b
			}
			// parse into jwt.Token to validate
			token, err := jwt.Parse(testToken)
			if err != nil {
				fmt.Printf("failed to parse access token contents: %v\n", err)
				return
			}
			// check if the token is invalid and we need a new one
			err = jwt.Validate(token)
			if err != nil {
				fmt.Printf("failed to validate access token...fetching a new one")
			} else {
				fmt.Printf("found a valid token...skipping login (use the '-f/--force' flag to login anyway)")
				return
			}
		}

		// start the login flow
		var err error
		accessToken, err = magellan.Login(loginUrl, targetHost, targetPort)
		if errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("\n=========================================\nServer closed.\n=========================================\n\n")
		} else if err != nil {
			fmt.Printf("failed to start server: %v\n", err)
		}

		// if we got a new token successfully, save it to the token path
		if accessToken != "" && tokenPath != "" {
			err := os.WriteFile(tokenPath, []byte(accessToken), os.ModePerm)
			if err != nil {
				fmt.Printf("failed to write access token to file: %v\n", err)
			}
		}
	},
}

func init() {
	loginCmd.Flags().StringVar(&loginUrl, "url", "http://127.0.0.1:3333/login", "set the login URL")
	loginCmd.Flags().StringVar(&targetHost, "target-host", "127.0.0.1", "set the target host to return the access code")
	loginCmd.Flags().IntVar(&targetPort, "target-port", 5000, "set the target host to return the access code")
	loginCmd.Flags().BoolVarP(&forceLogin, "force", "f", false, "start the login process even with a valid token")
	loginCmd.Flags().StringVar(&tokenPath, "token-path", ".ochami-token", "set the path the load/save the access token")
	loginCmd.Flags().BoolVar(&noBrowser, "no-browser", false, "prevent the default browser from being opened automatically")
	rootCmd.AddCommand(loginCmd)
}
