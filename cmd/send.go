package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	urlx "github.com/OpenCHAMI/magellan/internal/url"
	"github.com/OpenCHAMI/magellan/pkg/auth"
	"github.com/OpenCHAMI/magellan/pkg/client"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	sendDataArgs []string
)

var sendCmd = &cobra.Command{
	Use: "send [host]",
	Example: `  // send data from collect output
  magellan send -d @collected-1.json -d @collected-2.json https://smd.openchami.cluster
  magellan send -d '{...}' -d @collected-1.json https://api.exampe.com
	`,
	Short: "Send collected node information to specified host.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// make one request be host positional argument
		for _, host := range args {

			// concatenate all of the data from `-d` flag to send
			var (
				// smdClient = &client.SmdClient{Client: &http.Client{}}
				smdClient = client.NewClient[*client.SmdClient]()
				inputData []byte
				data      map[string]any
			)

			// load data either from file or directly from args
			for _, dataArg := range sendDataArgs {
				// determine if we're reading from file
				if len(dataArg) > 0 {
					// load from file
					if dataArg[0] == '@' {
						var (
							path     string = dataArg[1:]
							contents []byte
							err      error
						)
						contents, err = os.ReadFile(path)
						if err != nil {
							log.Error().Err(err).Str("path", path).Msg("failed to read file")
							continue
						}

						// unmarshal into map with specified format
						switch format {
						case "json":
							// make sure that the input provided is actually JSON
							isValid := json.Valid(inputData)
							if !isValid {
								log.Error().Err(err).
									Str("format", format).
									Str("hint", "Did you forget to set the input format with -F/--format?").
									Msg("input data is not valid JSON")
								os.Exit(1)
							}
						case "yaml":
							inputData, err = yamlToJson(inputData)
							if err != nil {
								log.Error().Err(err).Msg("failed to convert YAML input data to JSON")
								os.Exit(1)
							}
						}

						inputData = append(inputData, []byte(contents)...)
						fmt.Println("file:\n", string(contents))
					} else {
						// read data directly
						inputData = append(inputData, []byte(dataArg)...)
						fmt.Println("data:\n", string(dataArg))
					}
				} else {
					continue
				}
			}

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
				log.Debug().Str("path", cacertPath).Msg("using provided certificate path")
				err := client.LoadCertificateFromPath(smdClient, cacertPath)
				if err != nil {
					log.Warn().Err(err).Msg("could not load certificate")
				}
			}

			// create and set headers for request
			headers := client.HTTPHeader{}
			headers.Authorization(accessToken)
			headers.ContentType("application/json")

			host, err := urlx.Sanitize(host)
			if err != nil {
				log.Warn().Err(err).Str("host", host).Msg("could not sanitize host")
			}

			smdClient.URI = host
			err = smdClient.Add(inputData, headers)
			if err != nil {
				// try updating instead
				if forceUpdate {
					smdClient.Xname = data["ID"].(string)
					err = smdClient.Update(inputData, headers)
					if err != nil {
						log.Error().Err(err).Msgf("failed to forcibly update Redfish endpoint with ID %s", smdClient.Xname)
					}
				} else {
					log.Error().Err(err).Msgf("failed to add Redfish endpoint with ID %s", smdClient.Xname)
				}
			}
		}
	},
}

func init() {
	sendCmd.Flags().StringSliceVarP(&sendDataArgs, "data", "d", []string{}, "Set the data in to send to specified host.")
	sendCmd.Flags().StringVarP(&format, "format", "F", "json", "Set the data input format. (json|yaml)")
	sendCmd.Flags().BoolVarP(&forceUpdate, "force-update", "f", false, "Set flag to force update data sent to SMD.")
	sendCmd.Flags().StringVar(&cacertPath, "cacert", "", "Set the path to CA cert file. (defaults to system CAs when blank)")

	rootCmd.AddCommand(sendCmd)
}

func yamlToJson(input []byte) ([]byte, error) {
	var (
		data   map[string]any
		output []byte
		err    error
	)

	// unmarshal YAML contents into map
	err = yaml.Unmarshal(input, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML input data: %v", err)
	}

	// marshal map into JSON
	output, err = json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input data to JSON: %v", err)
	}
	return output, nil
}
