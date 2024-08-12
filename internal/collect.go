// Package magellan implements the core routines for the tools.
package magellan

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/OpenCHAMI/magellan/pkg/client"
	"github.com/OpenCHAMI/magellan/pkg/crawler"

	"github.com/OpenCHAMI/magellan/internal/util"
	"github.com/rs/zerolog/log"

	"github.com/Cray-HPE/hms-xname/xnames"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/stmcginnis/gofish"
	"golang.org/x/exp/slices"
)

// CollectParams is a collection of common parameters passed to the CLI
// for the 'collect' subcommand.
type CollectParams struct {
	URI         string // set by the 'host' flag
	Username    string // set the BMC username with the 'username' flag
	Password    string // set the BMC password with the 'password' flag
	Concurrency int    // set the of concurrent jobs with the 'concurrency' flag
	Timeout     int    // set the timeout with the 'timeout' flag
	CaCertPath  string // set the cert path with the 'cacert' flag
	Verbose     bool   // set whether to include verbose output with 'verbose' flag
	OutputPath  string // set the path to save output with 'output' flag
	ForceUpdate bool   // set whether to force updating SMD with 'force-update' flag
	AccessToken string // set the access token to include in request with 'access-token' flag
}

// This is the main function used to collect information from the BMC nodes via Redfish.
// The function expects a list of hosts found using the `ScanForAssets()` function.
//
// Requests can be made to several of the nodes using a goroutine by setting the q.Concurrency
// property value between 1 and 255.
func CollectInventory(assets *[]RemoteAsset, params *CollectParams) error {
	// check for available probe states
	if assets == nil {
		return fmt.Errorf("no assets found")
	}
	if len(*assets) <= 0 {
		return fmt.Errorf("no assets found")
	}

	// collect bmc information asynchronously
	var (
		offset     = 0
		wg         sync.WaitGroup
		found      = make([]string, 0, len(*assets))
		done       = make(chan struct{}, params.Concurrency+1)
		chanAssets = make(chan RemoteAsset, params.Concurrency+1)
		outputPath = path.Clean(params.OutputPath)
		smdClient  = client.NewClient[client.SmdClient](
			client.WithSecureTLS[client.SmdClient](params.CaCertPath),
		)
	)
	// set the client's host from the CLI param
	smdClient.URI = params.URI
	wg.Add(params.Concurrency)
	for i := 0; i < params.Concurrency; i++ {
		go func() {
			for {
				sr, ok := <-chanAssets
				if !ok {
					wg.Done()
					return
				}

				// generate custom xnames for bmcs
				// TODO: add xname customization via CLI
				node := xnames.Node{
					Cabinet:       1000,
					Chassis:       1,
					ComputeModule: 7,
					NodeBMC:       offset,
				}
				offset += 1

				// crawl BMC node to fetch inventory data via Redfish
				systems, err := crawler.CrawlBMC(crawler.CrawlerConfig{
					URI:      fmt.Sprintf("%s:%d", sr.Host, sr.Port),
					Username: params.Username,
					Password: params.Password,
					Insecure: true,
				})
				if err != nil {
					log.Error().Err(err).Msgf("failed to crawl BMC")
				}

				// data to be sent to smd
				data := map[string]any{
					"ID":                 fmt.Sprintf("%v", node.String()[:len(node.String())-2]),
					"Type":               "",
					"Name":               "",
					"FQDN":               sr.Host,
					"User":               params.Username,
					"MACRequired":        true,
					"RediscoverOnUpdate": false,
					"Systems":            systems,
				}

				// create and set headers for request
				headers := client.HTTPHeader{}
				headers.Authorization(params.AccessToken)
				headers.ContentType("application/json")

				body, err := json.MarshalIndent(data, "", "    ")
				if err != nil {
					log.Error().Err(err).Msgf("failed to marshal output to JSON")
				}

				if params.Verbose {
					fmt.Printf("%v\n", string(body))
				}

				// write JSON data to file if output path is set using hive partitioning strategy
				if outputPath != "" {
					// make directory if it does exists
					exists, err := util.PathExists(outputPath)
					if err == nil && !exists {
						err = os.MkdirAll(outputPath, 0o644)
						if err != nil {
							log.Error().Err(err).Msg("failed to make directory for output")
						} else {
							// make the output directory to store files
							outputPath, err := util.MakeOutputDirectory(outputPath, false)
							if err != nil {
								log.Error().Err(err).Msg("failed to make output directory")
							} else {
								// write the output to the final path
								err = os.WriteFile(path.Clean(fmt.Sprintf("%s/%s/%d.json", params.URI, outputPath, time.Now().Unix())), body, os.ModePerm)
								if err != nil {
									log.Error().Err(err).Msgf("failed to write data to file")
								}
							}
						}
					}
				}

				// add all endpoints to SMD ONLY if a host is provided
				if smdClient.URI != "" {
					err = smdClient.Add(body, headers)
					if err != nil {
						log.Error().Err(err).Msgf("failed to add Redfish endpoint")

						// try updating instead
						if params.ForceUpdate {
							smdClient.Xname = data["ID"].(string)
							err = smdClient.Update(body, headers)
							if err != nil {
								log.Error().Err(err).Msgf("failed to forcibly update Redfish endpoint")
							}
						}
					}
				} else {
					if params.Verbose {
						log.Warn().Msg("no request made (host argument is empty)")
					}
				}

				// got host information, so add to list of already probed hosts
				found = append(found, sr.Host)
			}
		}()
	}

	// use the found results to query bmc information
	for _, ps := range *assets {
		// skip if found info from host
		foundHost := slices.Index(found, ps.Host)
		if !ps.State || foundHost >= 0 {
			continue
		}
		chanAssets <- ps
	}

	// handle goroutine paths
	go func() {
		select {
		case <-done:
			wg.Done()
			break
		default:
			time.Sleep(1000)
		}
	}()

	close(chanAssets)
	wg.Wait()
	close(done)

	return nil
}
