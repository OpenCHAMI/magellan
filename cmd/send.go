package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/OpenCHAMI/magellan/internal/format"
	urlx "github.com/OpenCHAMI/magellan/internal/url"
	"github.com/OpenCHAMI/magellan/pkg/auth"
	"github.com/OpenCHAMI/magellan/pkg/client"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	sendInputFormat format.DataFormat = format.FORMAT_JSON
	sendDataArgs    []string
)

var sendCmd = &cobra.Command{
	Use: "send [data]",
	Example: `  // minimal working example
  magellan send -d @inventory.json https://smd.openchami.cluster

  // send data from multiple files (must specify -f/--format if not JSON)
  magellan send -d @cluster-1.json -d @cluster-2.json https://smd.openchami.cluster
  magellan send -d '{...}' -d @cluster-1.json https://proxy.example.com

  // send data to remote host by piping output of collect directly
  magellan collect -v -F yaml | magellan send -d @inventory.yaml -F yaml https://smd.openchami.cluster`,
	Short: "Send collected node information to specified host.",
	Args: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		// try to load access token either from env var, file, or config if var not set
		if accessToken == "" {
			var err error
			accessToken, err = auth.LoadAccessToken(tokenPath)
			if err != nil {
				log.Warn().Err(err).Msg("could not load access token")
			} else if accessToken != "" {
				log.Debug().Str("access_token", accessToken).Msg("using access token")
			}
		}

		// try and load cert if argument is passed for client
		var smdClient = client.NewSmdClient()
		if cacertPath != "" {
			log.Debug().Str("path", cacertPath).Msg("using provided certificate path")
			err := client.LoadCertificateFromPath(smdClient, cacertPath)
			if err != nil {
				log.Warn().Err(err).Str("path", cacertPath).Msg("could not load certificate")
			}
		}

		// make one request be host positional argument (restricted to 1 for now)
		var inputData []map[string]any
		temp := append(handleArgs(args), processDataArgs(sendDataArgs)...)
		for _, data := range temp {
			if data != nil {
				inputData = append(inputData, data)
			}
		}
		if len(inputData) == 0 {
			log.Error().Msg("data required with standard input or -d/--data flag")
			os.Exit(1)
		}

		// show the data that was just loaded as input
		log.Debug().Any("input", inputData).Send()

		for _, host := range args {
			var (
				body []byte
				err  error
			)

			smdClient.URI = host
			for _, dataObject := range inputData {
				// skip on to the next thing if it's does not exist
				if dataObject == nil {
					continue
				}

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
	},
}

func init() {
	sendCmd.Flags().StringArrayVarP(&sendDataArgs, "data", "d", []string{}, "Set the data to send to specified host (prepend @ for files)")
	sendCmd.Flags().VarP(&sendInputFormat, "format", "F", "Set the default data input format (json|yaml) can be overridden by file extension")
	sendCmd.Flags().BoolVarP(&forceUpdate, "force-update", "f", false, "Set flag to force update data sent to SMD")
	sendCmd.Flags().StringVar(&cacertPath, "cacert", "", "Set the path to CA cert file (defaults to system CAs when blank)")

	checkRegisterFlagCompletionError(sendCmd.RegisterFlagCompletionFunc("format", completionFormatData))
	rootCmd.AddCommand(sendCmd)
}

// processDataArgs takes a slice of strings that check for the @ symbol and loads
// the contents from the file specified in place (which replaces the path).
//
// NOTE: The purpose is to make the input arguments uniform for our request. This
// function is meant to handle data passed with the `-d/--data` flag and positional
// args from the CLI.
func processDataArgs(args []string) []map[string]any {
	// JSON representation
	type (
		JSONObject = map[string]any
		JSONArray  = []JSONObject
	)

	// load data either from file or directly from args
	var collection = make(JSONArray, len(args))
	for i, arg := range args {
		// if arg is empty string, then skip and continue
		if len(arg) > 0 {
			// determine if we're reading from file to load contents
			if strings.HasPrefix(arg, "@") {
				var (
					path     = strings.TrimLeft(arg, "@")
					contents []byte
					data     JSONArray
					err      error
				)
				contents, err = os.ReadFile(path)
				if err != nil {
					log.Error().Err(err).Str("path", path).Msg("failed to read file")
					continue
				}

				// skip empty files
				if len(contents) == 0 {
					log.Warn().Str("path", path).Msg("file is empty")
					continue
				}

				// convert/validate input data
				data, err = parseInput(contents, format.DataFormatFromFileExt(path, sendInputFormat))
				if err != nil {
					log.Error().Err(err).Str("path", path).Msg("failed to validate input from file")
				}

				// add loaded data to collection of all data
				collection = append(collection, data...)
			} else {
				// input should be a valid JSON
				var (
					data  JSONArray
					input = []byte(arg)
					err   error
				)
				if !json.Valid(input) {
					log.Error().Msgf("argument %d not a valid JSON", i)
					continue
				}
				err = json.Unmarshal(input, &data)
				if err != nil {
					log.Error().Err(err).Msgf("failed to unmarshal input for argument %d", i)
				}
				return data
			}
		}
	}
	return collection
}

func handleArgs(args []string) []map[string]any {
	// JSON representation
	type (
		JSONObject = map[string]any
		JSONArray  = []JSONObject
	)
	// no file to load, so we just use the joined args (since each one is a new line)
	// and then stop
	var (
		collection JSONArray
		data       []byte
		err        error
	)

	if len(sendDataArgs) > 0 {
		return nil
	}
	data, err = ReadStdin()
	if err != nil {
		log.Error().Err(err).Msg("failed to read from standard input")
		return nil
	}
	if len(data) == 0 {
		log.Warn().Msg("no data found from standard input")
		return nil
	}
	fmt.Println(string(data))
	collection, err = parseInput([]byte(data), sendInputFormat)
	if err != nil {
		log.Error().Err(err).Msg("failed to validate input from arg")
	}
	return collection
}

func parseInput(contents []byte, dataFormat format.DataFormat) ([]map[string]any, error) {
	var (
		data []map[string]any
		err  error
	)

	// convert/validate JSON input format
	err = format.UnmarshalData(contents, &data, dataFormat)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %v", err)
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
		if len(b) == 0 {
			break
		}
	}
	if err := input.Err(); err != nil {
		return b, fmt.Errorf("failed to read stdin: %w", err)
	}
	return b, nil
}
