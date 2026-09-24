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

  // send data from multiple files (must specify -f/--input-format if not JSON)
  magellan send -d @cluster-1.json -d @cluster-2.json https://smd.openchami.cluster
  magellan send -d '{...}' -d @cluster-1.json https://proxy.example.com

  // send data to remote host by piping output of collect directly
  magellan collect -v -F yaml | magellan send -d @inventory.yaml -f yaml https://smd.openchami.cluster`,
	Short: "Send collected node information to specified host.",
	Args: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		// the destination host is a required positional argument; without it
		// no request is attempted at all, which must not look like success
		if len(args) == 0 {
			log.Error().Msg("host argument required (e.g. magellan send ... https://smd.example.com)")
			os.Exit(1)
		}

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
		temp := append(handleArgs(args, sendInputFormat), processDataArgs(sendDataArgs, sendInputFormat)...)
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
		// inputRaw, _ := json.MarshalIndent(inputData, "", "  ")
		log.Debug().Int("endpoint_count", len(inputData)).Send()

		// deliver every data object; anything not delivered is counted so
		// this command exits non-zero instead of merely logging the failure
		sent, failed := sendDataToHosts(args, inputData, smdClient)
		log.Debug().Int("sent", sent).Int("failed", failed).Send()
		if failed > 0 {
			log.Error().
				Int("sent", sent).
				Int("failed", failed).
				Msgf("failed to send %d of %d request(s)", failed, sent+failed)
			os.Exit(1)
		}
	},
}

// sendDataToHosts delivers inputData to each host in hosts and reports how
// many data objects were successfully sent and how many were not.
//
// Every object that is not delivered counts as a failure: nil entries,
// hosts that cannot be parsed as a URL, marshalling errors, and requests
// rejected by the remote host (after the optional --force-update retry).
// Callers use the counts to exit non-zero on partial or total failure;
// previously such failures were only logged, so scripts and CI saw a
// successful exit even when nothing reached the remote host.
func sendDataToHosts(hosts []string, inputData []map[string]any, smdClient *client.SmdClient) (sent, failed int) {
	for _, host := range hosts {
		sanitized, err := urlx.Sanitize(host)
		if err != nil {
			log.Error().Err(err).Str("host", host).Msg("could not sanitize host")
			failed += len(inputData)
			continue
		}
		smdClient.URI = sanitized

		for _, dataObject := range inputData {
			// skip on to the next thing if it does not exist, but count it:
			// data that is not sent must not look like success
			if dataObject == nil {
				log.Warn().Str("host", sanitized).Msg("skipping request to host")
				failed++
				continue
			}

			// create and set headers for request
			headers := client.HTTPHeader{}
			headers.Authorization(accessToken)
			headers.ContentType("application/json")

			// convert to JSON to send data
			body, err := json.MarshalIndent(dataObject, "", "  ")
			if err != nil {
				log.Error().
					Err(err).
					Msg("failed to marshal request data")
				failed++
				continue
			}
			log.Debug().Str("host", sanitized).RawJSON("data", body).Send()

			// make request to remote host
			if err := smdClient.Add(body, headers); err != nil {
				// try updating instead
				if !forceUpdate {
					log.Error().
						Err(err).
						Str("host", sanitized).
						Str("ID", dataObjectID(dataObject)).
						Msg("failed to add Redfish endpoint")
					failed++
					continue
				}
				id, ok := dataObject["ID"].(string)
				if !ok || id == "" {
					log.Error().
						Str("host", sanitized).
						Msg("failed to forcibly update Redfish endpoint: data object is missing a string 'ID' field")
					failed++
					continue
				}
				smdClient.Xname = id
				if err := smdClient.Update(body, headers); err != nil {
					log.Error().
						Err(err).
						Str("host", sanitized).
						Str("ID", id).
						Msg("failed to forcibly update Redfish endpoint")
					failed++
					continue
				}
			}
			sent++
		}
	}
	return sent, failed
}

// dataObjectID returns the 'ID' field of dataObject as a string, or an
// empty string when it is missing or not a string.
func dataObjectID(dataObject map[string]any) string {
	if id, ok := dataObject["ID"].(string); ok {
		return id
	}
	return ""
}

func init() {
	sendCmd.Flags().StringArrayVarP(&sendDataArgs, "data", "d", []string{}, "Set the data to send to specified host (prepend @ for files)")
	sendCmd.Flags().VarP(&sendInputFormat, "input-format", "f", "Set the default data input format (json|yaml) can be overridden by file extension")
	sendCmd.Flags().BoolVar(&forceUpdate, "force-update", false, "Set flag to force update data sent to SMD")
	sendCmd.Flags().StringVar(&cacertPath, "cacert", "", "Set the path to CA cert file (defaults to system CAs when blank)")

	checkRegisterFlagCompletionError(sendCmd.RegisterFlagCompletionFunc("input-format", completionFormatData))
	rootCmd.AddCommand(sendCmd)
}

// processDataArgs takes a slice of strings that check for the @ symbol and loads
// the contents from the file specified in place (which replaces the path).
//
// NOTE: The purpose is to make the input arguments uniform for our request. This
// function is meant to handle data passed with the `-d/--data` flag and positional
// args from the CLI.
func processDataArgs(args []string, inputFormat format.DataFormat) []map[string]any {
	// JSON representation
	type (
		JSONObject = map[string]any
		JSONArray  = []JSONObject
	)

	// load data either from file or directly from args
	var collection = make(JSONArray, 0, len(args))
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
				data, err = parseInput(contents, format.DataFormatFromFileExt(path, inputFormat))
				if err != nil {
					log.Error().Err(err).Str("path", path).Msg("failed to validate input from file")
				}

				// add loaded data to collection of all data
				collection = append(collection, data...)
			} else {
				// input should be a valid data in the selected input format
				var (
					data  JSONArray
					input = []byte(arg)
					err   error
				)
				err = format.UnmarshalData(input, &data, inputFormat)
				if err != nil {
					log.Error().Err(err).Msgf("failed to unmarshal %s input for argument %d", inputFormat, i)
				}
				collection = append(collection, data...)
			}
		}
	}
	return collection
}

func handleArgs(args []string, inputFormat format.DataFormat) []map[string]any {
	// no file to load, so we just use the joined args (since each one is a new line)
	// and then stop
	type JSONArray = []map[string]any
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
	collection, err = parseInput([]byte(data), inputFormat)
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
