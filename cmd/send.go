package cmd

import (
	"bufio"
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
	Use: "send [data]",
	Example: `  // minimal working example
  magellan send -d @inventory.json --host https://smd.openchami.cluster

  // send data from multiple files (must specify -f/--format if not JSON)
  magellan send -d @cluster-1.json -d @cluster-2.json --host https://smd.openchami.cluster
  magellan send -d '{...}' -d @cluster-1.json --host https://proxy.example.com

  // send data to remote host by piping output of collect directly
  magellan collect -v -F yaml | magellan send -d @inventory.yaml -F yaml --host https://smd.openchami.cluster`,
	Short: "Send collected node information to specified host.",
	Args: func(cmd *cobra.Command, args []string) error {
		return nil
	},
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
		var inputData = append(processDataArgs(cmd, args), processDataArgs(cmd, sendDataArgs)...)
		if len(inputData) == 0 {
			log.Error().Msg("must include data using positional arg or -d/--data flag")
			fmt.Printf("args count: %d, data count: %d, size: %d", len(args), len(sendDataArgs), len(inputData))
			os.Exit(1)
		}
		for _, host := range hosts {
			var (
				body []byte
				err  error
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
					body, err = json.MarshalIndent(dataObject, "", "  ")
					if err != nil {
						log.Error().Err(err).Msg("failed to marshal request data")
						continue
					}

					if verbose {
						fmt.Println(string(body))
					}
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
	sendCmd.Flags().StringSliceVar(&hosts, "host", []string{}, "Set the host for the request")
	sendCmd.Flags().StringSliceVarP(&sendDataArgs, "data", "d", []string{}, "Set the data in to send to specified host")
	sendCmd.Flags().StringVarP(&sendInputFormat, "format", "F", FORMAT_JSON, "Set the data input format (json|yaml)")
	sendCmd.Flags().BoolVarP(&forceUpdate, "force-update", "f", false, "Set flag to force update data sent to SMD")
	sendCmd.Flags().StringVar(&cacertPath, "cacert", "", "Set the path to CA cert file (defaults to system CAs when blank)")

	sendCmd.MarkFlagRequired("host")
	rootCmd.AddCommand(sendCmd)
}

// processDataArgs takes a slice of strings that check for the @ symbol and loads
// the contents from the file specified in place (which replaces the path).
//
// NOTE: The purpose is to make the input arguments uniform for our request.
func processDataArgs(cmd *cobra.Command, args []string) [][]map[string]any {
	// load data either from file or directly from args
	type (
		JSONObject = map[string]any
		JSONArray  = []JSONObject
		DataArgs   = []JSONArray
	)

	if cmd.Flag("data").Changed {
		var newArgs = make(DataArgs, len(args))
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
						log.Error().Err(err).Str("path", path).Msg("failed to validate input from file")
					}

				}
			}
		}
		return newArgs
	} else {
		// no file to load, so we just use the joined args (since each one is a new line)
		// and then stop
		var (
			arr  JSONArray
			data []byte
			err  error
		)
		data, err = ReadStdin()
		if err != nil {
			log.Error().Err(err).Msg("faield to read from standard input")
			return nil
		}
		if len(data) == 0 {
			log.Warn().Msg("no data found from standard input")
			return nil
		}
		fmt.Println(string(data))
		arr, err = validateInput([]byte(data))
		if err != nil {
			log.Error().Err(err).Msg("failed to validate input from arg")
		}
		return DataArgs{arr}
	}
}

func validateInput(contents []byte) ([]map[string]any, error) {
	var (
		data []map[string]any
		err  error
	)
	// convert/validate JSON input format
	switch sendInputFormat {
	case FORMAT_JSON:
		err = json.Unmarshal(contents, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal input in JSON: %v", err)
		}
	case FORMAT_YAML:
		err = yaml.Unmarshal(contents, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal input in YAML: %v", err)
		}
	default:
		return nil, fmt.Errorf("unrecognized format")
	}
	return data, nil
}

// ReadStdin reads all of standard input and returns the bytes. If an error
// occurs during scanning, it is returned.
func ReadStdin() ([]byte, error) {
	var b []byte
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		b = append(b, input.Bytes()...)
		b = append(b, byte('\n'))
	}
	if err := input.Err(); err != nil {
		return b, fmt.Errorf("failed to read stdin: %w", err)
	}
	return b, nil
}
