// Package magellan implements the core routines for the tools.
package magellan

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/OpenCHAMI/magellan/pkg/client"
	"github.com/OpenCHAMI/magellan/pkg/crawler"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"gopkg.in/yaml.v3"

	"github.com/rs/zerolog/log"

	"github.com/Cray-HPE/hms-xname/xnames"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
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
	Format      string // set the output format
	ForceUpdate bool   // set whether to force updating SMD with 'force-update' flag
	AccessToken string // set the access token to include in request with 'access-token' flag
	SecretsFile string // set the path to secrets file
}

// This is the main function used to collect information from the BMC nodes via Redfish.
// The results of the collect are stored in a cache specified with the `--cache` flag.
// The function expects a list of hosts found using the `ScanForAssets()` function.
//
// Requests can be made to several of the nodes using a goroutine by setting the q.Concurrency
// property value between 1 and 10000.
func CollectInventory(assets *[]RemoteAsset, params *CollectParams, localStore secrets.SecretStore) ([]map[string]any, error) {
	// check for available remote assets found from scan
	if assets == nil {
		return nil, fmt.Errorf("no assets found")
	}
	if len(*assets) <= 0 {
		return nil, fmt.Errorf("no assets found")
	}

	// collect bmc information asynchronously
	var (
		offset     = 0
		wg         sync.WaitGroup
		collection = make([]map[string]any, 0)
		found      = make([]string, 0, len(*assets))
		done       = make(chan struct{}, params.Concurrency+1)
		chanAssets = make(chan RemoteAsset, params.Concurrency+1)
		outputPath = path.Clean(params.OutputPath)
		smdClient  = &client.SmdClient{Client: &http.Client{}}
	)

	// set the client's params from CLI
	// NOTE: temporary solution until client.NewClient() is fixed
	smdClient.URI = params.URI
	if params.CaCertPath != "" {
		cacert, err := os.ReadFile(params.CaCertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert path: %w", err)
		}
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(cacert)
		smdClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            certPool,
				InsecureSkipVerify: true,
			},
			DisableKeepAlives: true,
			Dial: (&net.Dialer{
				Timeout:   120 * time.Second,
				KeepAlive: 120 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   120 * time.Second,
			ResponseHeaderTimeout: 120 * time.Second,
		}
	}
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
				var (
					uri  = fmt.Sprintf("%s:%d", sr.Host, sr.Port)
					node = xnames.Node{
						Cabinet:       1000,
						Chassis:       1,
						ComputeModule: 7,
						NodeBMC:       offset,
					}
				)
				offset += 1

				// crawl BMC node to fetch inventory data via Redfish
				var (
					fallbackStore = secrets.NewStaticStore(params.Username, params.Password)
					systems       []crawler.InventoryDetail
					managers      []crawler.Manager
					config        = crawler.CrawlerConfig{
						URI:             uri,
						CredentialStore: localStore,
						Insecure:        true,
						UseDefault:      true,
					}
					err error
				)

				// determine if local store exists and has credentials for
				// the provided secretID...
				// if it does not, use the fallback static store instead with
				// the username and password provided as arguments
				if localStore != nil {
					_, err := localStore.GetSecretByID(uri)
					if err != nil {
						log.Warn().Err(err).Msgf("could not retrieve secrets for '%s'...falling back to credentials provided with flags -u/-p for user '%s'", uri, params.Username)
						if params.Username != "" && params.Password != "" {
							config.CredentialStore = fallbackStore
						} else if !config.UseDefault {
							log.Warn().Msgf("no fallback credentials provided for '%s'", params.Username)
							continue
						}
					}
				} else {
					log.Warn().Msgf("invalid store for %s...falling back to default provided credentials for user '%s'", uri, params.Username)
					config.CredentialStore = fallbackStore
				}

				// crawl for node and BMC information
				systems, err = crawler.CrawlBMCForSystems(config)
				if err != nil {
					log.Error().Err(err).Msg("failed to crawl BMC for systems")
				}
				managers, err = crawler.CrawlBMCForManagers(config)
				if err != nil {
					log.Error().Err(err).Msg("failed to crawl BMC for managers")
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
					"Managers":           managers,
					"SchemaVersion":      1,
				}

				// optionally, add the MACAddr property if we find a matching IP
				// from the correct ethernet interface
				mac, err := FindMACAddressWithIP(config, net.ParseIP(sr.Host))
				if err != nil {
					log.Warn().Err(err).Msgf("failed to find MAC address with IP '%s'", sr.Host)
				}
				if mac != "" {
					data["MACAddr"] = mac
				}

				// create and set headers for request
				headers := client.HTTPHeader{}
				headers.Authorization(params.AccessToken)
				headers.ContentType("application/json")

				var body []byte
				switch params.Format {
				case "json":
					body, err = json.MarshalIndent(data, "", "    ")
					if err != nil {
						log.Error().Err(err).Msgf("failed to marshal output to JSON")
					}
				case "yaml":
					body, err = yaml.Marshal(data)
					if err != nil {
						log.Error().Err(err).Msgf("failed to marshal output to YAML")
					}
				}

				if params.Verbose {
					fmt.Printf("%v\n", string(body))
				}

				// add data output to collections
				collection = append(collection, data)

				// write data to file if output path is set using set format
				if outputPath != "" {
					switch params.Format {
					case "hive":
						var (
							finalPath = fmt.Sprintf("./%s/%s/%d.json", outputPath, data["ID"], time.Now().Unix())
							finalDir  = filepath.Dir(finalPath)
						)
						// if it doesn't, make the directory and write file
						err = os.MkdirAll(finalDir, 0o777)
						if err == nil { // no error
							err = os.WriteFile(path.Clean(finalPath), body, os.ModePerm)
							if err != nil {
								log.Error().Err(err).Msgf("failed to write collect output to file")
							}
						} else { // error is set
							log.Error().Err(err).Msg("failed to make directory for collect output")
						}
					case "json":
					case "yaml":

					default:
					}
				}

				// add all endpoints to SMD ONLY if a host is provided
				if smdClient.URI != "" {
					err = smdClient.Add(body, headers)
					if err != nil {

						// try updating instead
						if params.ForceUpdate {
							smdClient.Xname = data["ID"].(string)
							err = smdClient.Update(body, headers)
							if err != nil {
								log.Error().Err(err).Msgf("failed to forcibly update Redfish endpoint")
							}
						} else {
							log.Error().Err(err).Msgf("failed to add Redfish endpoint")
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

	return collection, nil
}

// FindMACAddressWithIP() returns the MAC address of an ethernet interface with
// a matching IPv4Address. Returns an empty string and error if there are no matches
// found.
func FindMACAddressWithIP(config crawler.CrawlerConfig, targetIP net.IP) (string, error) {
	// get the managers to find the BMC MAC address compared with IP
	//
	// NOTE: Since we don't have a RedfishEndpoint type abstraction in
	// magellan and the crawler crawls for systems information, it
	// may just make more sense to get the managers directly via
	// gofish (at least for now). If there's a need for grabbing more
	// manager information in the future, we can move the logic into
	// the crawler.
	bmc_creds, err := config.GetUserPass()
	if err != nil {
		return "", fmt.Errorf("failed to get credentials for URI: %s", config.URI)
	}

	client, err := gofish.Connect(gofish.ClientConfig{
		Endpoint:  config.URI,
		Username:  bmc_creds.Username,
		Password:  bmc_creds.Password,
		Insecure:  config.Insecure,
		BasicAuth: true,
	})
	if err != nil {
		if strings.HasPrefix(err.Error(), "404:") {
			err = fmt.Errorf("no ServiceRoot found.  This is probably not a BMC: %s", config.URI)
		}
		if strings.HasPrefix(err.Error(), "401:") {
			err = fmt.Errorf("authentication failed.  Check your username and password: %s", config.URI)
		}
		event := log.Error()
		event.Err(err)
		event.Msg("failed to connect to BMC")
		return "", err
	}
	defer client.Logout()

	var (
		rf_service  = client.GetService()
		rf_managers []*redfish.Manager
	)
	rf_managers, err = rf_service.Managers()
	if err != nil {
		return "", fmt.Errorf("failed to get managers: %v", err)
	}

	// find the manager with the same IP address of the BMC to get
	// it's MAC address from its EthernetInterface
	for _, manager := range rf_managers {
		eths, err := manager.EthernetInterfaces()
		if err != nil {
			log.Error().Err(err).Msgf("failed to get ethernet interfaces from manager '%s'", manager.Name)
			continue
		}
		for _, eth := range eths {
			// compare the ethernet interface IP with argument
			for _, ip := range eth.IPv4Addresses {
				if ip.Address == targetIP.String() {
					// we found matching IP address so return the ethernet interface MAC
					return eth.MACAddress, nil
				}
			}
			// do the same thing as above, but with static IP addresses
			for _, ip := range eth.IPv4StaticAddresses {
				if ip.Address == targetIP.String() {
					return eth.MACAddress, nil
				}
			}
			// no matches found, so go to next ethernet interface
			continue
		}
	}
	// no matches found, so return an empty string
	return "", fmt.Errorf("no ethernet interfaces found with IP address")
}
