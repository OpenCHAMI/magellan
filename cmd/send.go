package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	urlx "github.com/OpenCHAMI/magellan/internal/url"
	"github.com/OpenCHAMI/magellan/pkg/auth"
	"github.com/OpenCHAMI/magellan/pkg/client"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	sendInputFormat string
	sendDataArgs    []string
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
		// try to load access token either from env var, file, or config if var not set
		if accessToken == "" {
			var err error
			accessToken, err = auth.LoadAccessToken(tokenPath)
			if err != nil && verbose {
				log.Warn().Err(err).Msgf("could not load access token")
			} else if debug && accessToken != "" {
				log.Debug().Str("access_token", accessToken).Msg("using access token")
			}
		}

		// try and load cert if argument is passed for client
		var smdClient = client.NewSmdClient()
		if cacertPath != "" {
			log.Debug().Str("path", cacertPath).Msg("using provided certificate path")
			err := client.LoadCertificateFromPath(smdClient, cacertPath)
			if err != nil {
				log.Warn().Err(err).Msg("could not load certificate")
			}
		}

		// make one request be host positional argument (restricted to 1 for now)
		for _, host := range args {
			var (
				inputData = processDataArgs(sendDataArgs)
				body      []byte
				err       error
			)

			smdClient.URI = host
			for _, dataArray := range inputData {
				for _, dataObject := range dataArray {

					// create and set headers for request
					headers := client.HTTPHeader{}
					headers.Authorization(accessToken)
					headers.ContentType("application/json")

					host, err = urlx.Sanitize(host)
					if err != nil {
						log.Warn().Err(err).Str("host", host).Msg("could not sanitize host")
					}

					// convert to JSON to send data
					body, err = json.Marshal(dataObject)
					if err != nil {
						log.Error().Err(err).Msg("failed to marshal request data")
						continue
					}

					fmt.Println(string(body))
					err = smdClient.Add(body, headers)
					if err != nil {
						// try updating instead
						if forceUpdate {
							smdClient.Xname = dataObject["ID"].(string)
							err = smdClient.Update(body, headers)
							if err != nil {
								log.Error().Err(err).Msgf("failed to forcibly update Redfish endpoint with ID %s", smdClient.Xname)
							}
						} else {
							log.Error().Err(err).Msgf("failed to add Redfish endpoint with ID %s", smdClient.Xname)
						}
					}
				}
			}
		}
	},
}

func init() {
	sendCmd.PersistentFlags().StringSliceVarP(&sendDataArgs, "data", "d", []string{}, "Set the data in to send to specified host")
	sendCmd.PersistentFlags().StringVarP(&sendInputFormat, "format", "F", FORMAT_JSON, "Set the data input format (json|yaml)")
	sendCmd.PersistentFlags().BoolVarP(&forceUpdate, "force-update", "f", false, "Set flag to force update data sent to SMD")
	sendCmd.PersistentFlags().StringVar(&cacertPath, "cacert", "", "Set the path to CA cert file (defaults to system CAs when blank)")

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

// processDataArgs takes a slice of strings that check for the @ symbol and loads
// the contents from the file specified in place (which replaces the path).
//
// NOTE: The purpose is to make the input arguments uniform for our request.
func processDataArgs(args []string) [][]map[string]any {
	// load data either from file or directly from args
	type (
		JSONObject = map[string]any
		JSONArray  = []JSONObject
		DataArgs   = []JSONArray
	)
	var (
		newArgs = make(DataArgs, len(args))
		err     error
	)
	for i, arg := range args {
		// if arg is empty string, then continue
		if len(arg) > 0 {
			// determine if we're reading from file to load contents
			if strings.HasPrefix(arg, "@") {
				var (
					path     string = strings.TrimLeft(arg, "@")
					contents []byte
					err      error
				)

				contents, err = os.ReadFile(path)
				if err != nil {
					log.Error().Err(err).Str("path", path).Msg("failed to read file")
					continue
				}

				// convert/validate JSON input format
				newArgs[i], err = validateInput(contents)
				if err != nil {
					log.Error().Err(err).Msg("failed to validate input")
				}

			} else {

				// nothing to load, so we just use the arg itself
				newArgs[i], err = validateInput([]byte(arg))
				if err != nil {
					log.Error().Err(err).Msg("failed to validate input")
				}
			}
		} else {
			continue
		}
	}
	return newArgs
}

func validateInput(contents []byte) ([]map[string]any, error) {
	var (
		data []map[string]any
		err  error
	)
	// convert/validate JSON input format
	switch sendInputFormat {
	case FORMAT_JSON:
		fmt.Println(string(contents))
		err = json.Unmarshal(contents, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal input in JSON")
		}
	case FORMAT_YAML:
		err = yaml.Unmarshal(contents, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal input in YAML")
		}
	}
	return data, nil
}
