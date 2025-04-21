package cmd

import (
	"crypto/x509"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	urlx "github.com/OpenCHAMI/magellan/internal/url"
	"github.com/OpenCHAMI/magellan/pkg/auth"
	"github.com/OpenCHAMI/magellan/pkg/client"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var sendData []string

var sendCmd = &cobra.Command{
	Use: "send [host]",
	Example: `  // send data from collect output
  magellan send -d @collected-1.json -d @collected-2.json https://smd.openchami.cluster
  magellan send -d '{...}' -d @collected-1.json https://api.exampe.com
	`,
	Short: "Send collected node information to specified host.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// concatenate all of the data from `-d` flag to send
		var (
			smdClient = &client.SmdClient{Client: &http.Client{}}
			inputData = []byte(strings.Join(sendData, "\n"))
		)

		// try to load access token either from env var, file, or config if var not set
		if accessToken == "" {
			var err error
			accessToken, err = auth.LoadAccessToken(tokenPath)
			if err != nil && verbose {
				log.Warn().Err(err).Msgf("could not load access token")
			}
		}

		// try and load cert if argument is passed
		if cacertPath != "" {
			cacert, err := os.ReadFile(cacertPath)
			if err != nil {
				log.Warn().Err(err).Msg("failed to read cert path")
			}
			certPool := x509.NewCertPool()
			certPool.AppendCertsFromPEM(cacert)
			// smdClient.WithCertPool(certPool)
			// client.WithCertPool(smdClient, certPool)
		}

		// create and set headers for request
		headers := client.HTTPHeader{}
		headers.Authorization(accessToken)
		headers.ContentType("application/json")

		// unmarshal into map
		data := map[string]any{}
		err := json.Unmarshal(inputData, &data)
		if err != nil {
			log.Error().Err(err).Msg("failed to unmarshal data to make request")
		}

		for _, host := range args {
			host, err := urlx.Sanitize(host)
			if err != nil {
				log.Error().Err(err).Msg("failed to sanitize host")
			}

			smdClient.URI = host
			err = smdClient.Add(inputData, headers)
			if err != nil {

				// try updating instead
				if forceUpdate {
					smdClient.Xname = data["ID"].(string)
					err = smdClient.Update(inputData, headers)
					if err != nil {
						log.Error().Err(err).Msgf("failed to forcibly update Redfish endpoint")
					}
				} else {
					log.Error().Err(err).Msgf("failed to add Redfish endpoint")
				}
			}
		}
	},
}

func init() {
	sendCmd.Flags().StringSliceVarP(&sendData, "data", "d", []string{}, "Set the data to send to specified host.")
	sendCmd.Flags().BoolVarP(&forceUpdate, "force-update", "f", false, "Set flag to force update data sent to SMD.")
	sendCmd.Flags().StringVar(&cacertPath, "cacert", "", "Set the path to CA cert file. (defaults to system CAs when blank)")

	rootCmd.AddCommand(sendCmd)
}
