package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in with identity provider for access token",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		// check if we have a valid JWT before starting login
		if !viper.GetBool("login.force") {
			// parse into jwt.Token to validate
			token, err := jwt.Parse([]byte(viper.GetString("access-token")))
			if err != nil {
				log.Error().Err(err).Msgf("failed to parse access token contents")
				return
			}
			// check if the token is invalid and we need a new one
			err = jwt.Validate(token)
			if err != nil {
				log.Error().Err(err).Msgf("failed to validate access token...fetching a new one")
			} else {
				log.Printf("found a valid token...skipping login (use the '-f/--force' flag to login anyway)")
				return
			}
		}

		targetHost := viper.GetString("login.target-host")
		targetPort := viper.GetInt("login.target-port")
		if viper.GetBool("verbose") {
			log.Printf("Listening for token on %s:%d", targetHost, targetPort)
		}

		// start the login flow
		var err error
		accessToken, err := magellan.Login(viper.GetString("login.url"), targetHost, targetPort)
		if errors.Is(err, http.ErrServerClosed) {
			if viper.GetBool("verbose") {
				fmt.Printf("\n=========================================\nServer closed.\n=========================================\n\n")
			}
		} else if err != nil {
			log.Error().Err(err).Msgf("failed to start server")
		}

		// if we got a new token successfully, save it to the token path
		tokenPath := viper.GetString("token-path")
		if accessToken != "" && tokenPath != "" {
			err := os.WriteFile(tokenPath, []byte(accessToken), os.ModePerm)
			if err != nil {
				log.Error().Err(err).Msgf("failed to write access token to file")
			}
		}
	},
}

func init() {
	addFlag("login.url", loginCmd, "url", "", "http://127.0.0.1:3333/login", "Set the login URL")
	addFlag("login.target-host", loginCmd, "target-host", "", "127.0.0.1", "Set the target host to return the access code")
	addFlag("login.target-port", loginCmd, "target-port", "", 5000, "Set the target host to return the access code")
	addFlag("login.force", loginCmd, "force", "f", false, "Start the login process even with a valid token")
	addFlag("login.no-browser", loginCmd, "no-browser", "", false, "Prevent the default browser from being opened automatically")

	rootCmd.AddCommand(loginCmd)
}
