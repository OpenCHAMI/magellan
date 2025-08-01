// Package magellan implements the core routines for the tools.
package magellan

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/OpenCHAMI/magellan/internal/util"
	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/OpenCHAMI/magellan/pkg/client"
	"github.com/OpenCHAMI/magellan/pkg/crawler"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"gopkg.in/yaml.v3"

	"github.com/rs/zerolog/log"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
	"golang.org/x/exp/slices"
)

// CollectParams is a collection of common parameters passed to the CLI
// for the 'collect' subcommand.
type CollectParams struct {
	Concurrency int                 // set the of concurrent jobs with the 'concurrency' flag
	Timeout     int                 // set the timeout with the 'timeout' flag
	CaCertPath  string              // set the cert path with the 'cacert' flag
	Verbose     bool                // set whether to include verbose output with 'verbose' flag
	OutputPath  string              // set the path to save output with 'output' flag
	OutputDir   string              // set the directory path to save output with `output-dir` flag
	Format      string              // set the output format
	ForceUpdate bool                // set whether to force updating SMD with 'force-update' flag
	AccessToken string              // set the access token to include in request with 'access-token' flag
	BMCIdMap   string              // Set the path to the BMC ID mapping YAML or JSON file (if any)
	SecretStore secrets.SecretStore // set BMC credentials
}

// BMCIdMap contains the mapping of host address strings to BMC Identifiers
// supplied by the --bmc-id-map option to collect. IdMap is the mapping itself,
// MapKey specifies what string to use as the key to the map. For now, that is
// always 'bmc-ip-addr'. In the future other options may be available.
type BMCIdMap struct {
	IdMap      map[string]string `json:"id_map" yaml:"id_map"`
	MapKey     string `json:"map_key" yaml:"map_key"`
}

// This is the main function used to collect information from the BMC nodes via Redfish.
// The results of the collect are stored in a cache specified with the `--cache` flag.
// The function expects a list of hosts found using the `ScanForAssets()` function.
//
// Requests can be made to several of the nodes using a goroutine by setting the q.Concurrency
// property value between 1 and 10000.
func CollectInventory(assets *[]RemoteAsset, params *CollectParams) ([]map[string]any, error) {
	// check for available remote assets found from scan
	if assets == nil {
		return nil, fmt.Errorf("no assets found")
	}
	if len(*assets) <= 0 {
		return nil, fmt.Errorf("no assets found")
	}

	// collect bmc information asynchronously
	var (
		wg         sync.WaitGroup
		collection = make([]map[string]any, 0)
		found      = make([]string, 0, len(*assets))
		done       = make(chan struct{}, params.Concurrency+1)
		chanAssets = make(chan RemoteAsset, params.Concurrency+1)
		bmcIdMap   *BMCIdMap
		err        error
	)
	// Get the host to BMC ID mapping
	bmcIdMap, err = getBMCIdMap(params.BMCIdMap, params.Format)
	if err != nil {
		return nil, err
	}
	// Validate the MapKey field in the ID Map if a map was found
	// (the only value currently allowed is 'bmc-ip-addr', but
	// this is where any other legal values would be added).
	if bmcIdMap != nil {
		switch bmcIdMap.MapKey {
		case "bmc-ip-addr":
			break
		default:
			return nil, fmt.Errorf("invalid 'map_key' field '%s' in BMC ID Map a valid value is 'bmc-ip-addr", bmcIdMap.MapKey)
		}
	} else {
		log.Warn().Msg("no BMC ID Map (--bmc-id-map string option) provided, BMC IDs will be IP addresses which are incompatible with SMD")
	}

	// set the client's params from CLI
	wg.Add(params.Concurrency)
	for i := 0; i < params.Concurrency; i++ {
		go func() {
			for {
				sr, ok := <-chanAssets
				if !ok {
					wg.Done()
					return
				}

				trimmedHost := strings.TrimPrefix(sr.Host, "https://")
				uri  := fmt.Sprintf("%s:%d", sr.Host, sr.Port)
				bmcId := getBMCId(bmcIdMap, trimmedHost)

				// If bmcId is empty, skip this BMC. Empty means that there
				// is a valid mapping, but there was no match for this host
				// in the mapping, meaning that the BMC is unrecognized. Skip
				// this BMC.
				if bmcId == "" {
					continue
				}

				// crawl BMC node to fetch inventory data via Redfish
				var (
					systems  []crawler.InventoryDetail
					managers []crawler.Manager
					config   = crawler.CrawlerConfig{
						URI:             uri,
						CredentialStore: params.SecretStore,
						Insecure:        true,
						UseDefault:      true,
					}
				)

				// crawl for node and BMC information
				systems, err = crawler.CrawlBMCForSystems(config)
				if err != nil {
					log.Error().Err(err).Str("uri", uri).Msg("failed to crawl BMC for systems")
				}
				managers, err = crawler.CrawlBMCForManagers(config)
				if err != nil {
					log.Error().Err(err).Str("uri", uri).Msg("failed to crawl BMC for managers")
				}

				// we didn't find anything so do not proceed
				if util.IsEmpty(systems) && util.IsEmpty(managers) {
					continue
				}

				// get BMC username to send
				bmcCreds := bmc.GetBMCCredentialsOrDefault(params.SecretStore, config.URI)
				if bmcCreds == (bmc.BMCCredentials{}) {
					log.Warn().Str("id", config.URI).Msg("username will be blank")
				}

				// data to be sent to smd
				data := map[string]any{
					"ID":                 bmcId,
					"Type":               "",
					"Name":               "",
					"FQDN":               strings.TrimPrefix(sr.Host, "https://"),
					"User":               bmcCreds.Username,
					"MACRequired":        true,
					"RediscoverOnUpdate": false,
					"Systems":            systems,
					"Managers":           managers,
					"SchemaVersion":      1,
				}

				// optionally, add the MACAddr property if we find a matching IP
				// from the correct ethernet interface

				host := sr.Host
				str_protocol := "https://"
				if strings.Contains(host, str_protocol) {
					host = strings.TrimPrefix(sr.Host, str_protocol)
				}
				mac, err := FindMACAddressWithIP(config, net.ParseIP(host))
				if err != nil {
					log.Warn().Err(err).Msgf("failed to find MAC address with IP '%s'", host)
				}
				if mac != "" {
					data["MACAddr"] = mac
				}

				// create and set headers for request
				headers := client.HTTPHeader{}
				headers.Authorization(params.AccessToken)
				headers.ContentType("application/json")

				// add data output to collections
				collection = append(collection, data)

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

	var (
		output []byte
	)

	// format our output to write to file or standard out
	format := util.DataFormatFromFileExt(params.OutputPath, params.Format)
	switch format {
	case util.FORMAT_JSON:
		output, err = json.MarshalIndent(collection, "", "    ")
		if err != nil {
			log.Error().Err(err).Msgf("failed to marshal output to JSON")
		}
	case util.FORMAT_YAML:
		output, err = yaml.Marshal(collection)
		if err != nil {
			log.Error().Err(err).Msgf("failed to marshal output to YAML")
		}
	}

	// print the final combined output at the end to write to file
	if params.Verbose {
		fmt.Printf("%v\n", string(output))
	}

	// write data to file in preset directory if output path is set using set format
	if params.OutputDir != "" {
		for _, data := range collection {
			var (
				finalPath = fmt.Sprintf("./%s/%s/%d.%s", path.Clean(params.OutputDir), data["ID"], time.Now().Unix(), format)
				finalDir  = filepath.Dir(finalPath)
			)
			// if it doesn't, make the directory and write file
			err = os.MkdirAll(finalDir, 0o777)
			if err == nil { // no error
				err = os.WriteFile(path.Clean(finalPath), output, os.ModePerm)
				if err != nil {
					log.Error().Err(err).Msgf("failed to write collect output to file")
				}
			} else { // error is set
				log.Error().Err(err).Msg("failed to make directory for collect output")
			}
		}
	}

	// write data to only to the path set (no preset directory structure)
	if params.OutputPath != "" {
		// if it doesn't, make the directory and write file
		err = os.MkdirAll(filepath.Dir(params.OutputPath), 0o777)
		if err == nil { // no error
			err = os.WriteFile(path.Clean(params.OutputPath), output, os.ModePerm)
			if err != nil {
				log.Error().Err(err).Msgf("failed to write collect output to file")
			}
		} else { // error is set
			log.Error().Err(err).Msg("failed to make directory for collect output")
		}
	}

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

func getBMCIdMap(data string, format string)(*BMCIdMap, error) {
	// If no mapping is provided, there is no error, but there is
	// also no mapping, just return nil with no error and let the
	// caller pass that around.
	if data == "" {
		return nil, nil
	}

	var bmcIdMap BMCIdMap
	// First, check whether 'data' specifies a file (i.e. starts
	// with '@'). If not, it should be a JSON string containing the
	// map data. Otherwise, strip the '@' and fall through.
	if data[0] != '@' {
		err := json.Unmarshal([]byte(data), &bmcIdMap)
		if err != nil {
			return nil, err
		}
		return &bmcIdMap, nil
	}

	// The map data is in a file. Get the path from what comes
	// after the '@' and process it.
	path := data[1:]

	// Read in the contents of the map file, since we are going to
	// do that no matter what type it is...
	input, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading BMC ID mapping file '%s': %v", path, err)
	}

	// Decode the file based on the appropriate format.
	switch util.DataFormatFromFileExt(path, format) {
	case util.FORMAT_JSON:
		// Read in JSON file
		err := json.Unmarshal(input, &bmcIdMap)
		if err != nil {
			return nil, err
		}
	case util.FORMAT_YAML:
		// Read in YAML file
		err := yaml.Unmarshal(input, &bmcIdMap)
		if err != nil {
			return nil, err
		}
	}
	return &bmcIdMap, nil
}

// Generate a BMC ID string associated with 'selector' in the provided
// 'BMCIdMap'. If there is no map, then return the selector string
// itself.  If the map is present but the host is not present in the
// map, then log a warning and return an empty string indicating that
// the BMC ID was not composed.
func getBMCId(bmcIdMap *BMCIdMap, selector string) (string) {
	if bmcIdMap == nil {
		return selector
	}
	// Go does not error out on string map references that do not
	// match the selector, it simply produces an empty
	// string. Recognize that case and log it, then return an
	// empty string.
	bmcId := bmcIdMap.IdMap[selector]
	if bmcId == "" {
		log.Warn().Msgf("no mapping found from host selector '%v' to a BMC ID", selector)
		return ""
	}
	return bmcId
}
