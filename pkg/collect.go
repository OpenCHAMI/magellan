// Package magellan implements the core routines for the tools.
package magellan

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/openchami/magellan/internal/format"
	"github.com/openchami/magellan/internal/util"
	"github.com/openchami/magellan/pkg/bmc"
	"github.com/openchami/magellan/pkg/crawler"
	"github.com/openchami/magellan/pkg/idmap"
	"github.com/openchami/magellan/pkg/models"
	"github.com/openchami/magellan/pkg/secrets"

	"github.com/rs/zerolog/log"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stmcginnis/gofish/schemas"
)

// CollectParams is a collection of common parameters passed to the CLI
// for the 'collect' subcommand.
type CollectParams struct {
	Concurrency  int                 // set the of concurrent jobs with the 'concurrency' flag
	Timeout      int                 // set the timeout with the 'timeout' flag
	Insecure     bool                // set whether to ignore TLS verification
	OutputPath   string              // set the path to save output with 'output' flag
	OutputDir    string              // set the directory path to save output with `output-dir` flag
	OutputFormat format.DataFormat   // set the output format
	InputFormat  format.DataFormat   // set the input format
	BMCIDMap     string              // Set the path to the BMC ID mapping YAML or JSON data or file name (if any)
	SecretStore  secrets.SecretStore // set BMC credentials
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
		return nil, fmt.Errorf("invalid assets (assets == nil)")
	}
	if len(*assets) <= 0 {
		return nil, fmt.Errorf("no assets found (len(*assets) == 0)")
	}
	if params == nil {
		return nil, fmt.Errorf("invalid collection parameters (params == nil)")
	}
	if params.Concurrency <= 0 {
		return nil, fmt.Errorf("invalid concurrency: %d", params.Concurrency)
	}
	mapper, err := idmap.NewIDMapper(params.BMCIDMap, params.OutputFormat)
	if err != nil {
		return nil, fmt.Errorf("invalid BMC ID mapping: check mapping file and format")
	}
	for index, asset := range *assets {
		host, err := url.ParseRequestURI(asset.Host)
		if err != nil || host.Hostname() == "" || host.User != nil || host.RawQuery != "" || host.Path != "" || host.Port() != "" || (host.Scheme != "http" && host.Scheme != "https") || asset.Port < 1 || asset.Port > 65535 {
			return nil, fmt.Errorf("invalid collect asset at index %d: expected an HTTP(S) host and a valid port", index)
		}
	}

	// collect bmc information asynchronously
	var (
		wg           sync.WaitGroup
		collection   = make([]map[string]any, 0)
		failures     []error
		collectionMu sync.Mutex
		chanAssets   = make(chan RemoteAsset, params.Concurrency+1)
	)

	// set the client's params from CLI
	wg.Add(params.Concurrency)
	for i := 0; i < params.Concurrency; i++ {
		go func() {
			defer wg.Done()
			for {
				sr, ok := <-chanAssets
				if !ok {
					return
				}

				endpoint, _ := url.ParseRequestURI(sr.Host)
				trimmedHost := endpoint.Hostname()
				uri := fmt.Sprintf("%s:%d", sr.Host, sr.Port)
				keys := &idmap.MapperKeys{
					IPv4Addr: trimmedHost,
				}
				bmcID := mapper.GetMappedID(keys)

				// If bmcID is empty, skip this
				// BMC. Empty means that there is a
				// valid mapping, but there was no
				// match for this host in the mapping,
				// meaning that the BMC is
				// unrecognized. Skip this BMC.
				if bmcID == "" {
					if params.BMCIDMap == "" {
						collectionMu.Lock()
						failures = append(failures, fmt.Errorf("BMC %s: cannot generate an ID; provide a BMC ID map", trimmedHost))
						collectionMu.Unlock()
					}
					continue
				}

				// crawl BMC node to fetch inventory data via Redfish
				var (
					systems  []models.InventoryDetail
					managers []models.Manager
					config   = crawler.CrawlerConfig{
						URI:             uri,
						CredentialStore: params.SecretStore,
						Insecure:        params.Insecure,
						UseDefault:      true,
					}
				)

				// crawl for node and BMC information
				systems, systemsErr := crawler.CrawlBMCForSystems(config)
				managers, managersErr := crawler.CrawlBMCForManagers(config)
				collectionMu.Lock()
				if systemsErr != nil {
					failures = append(failures, fmt.Errorf("BMC %s: failed to collect systems (check credentials and Redfish access)", bmcID))
				}
				if managersErr != nil {
					failures = append(failures, fmt.Errorf("BMC %s: failed to collect managers (check credentials and Redfish access)", bmcID))
				}
				if systemsErr == nil && managersErr == nil && util.IsEmpty(systems) && util.IsEmpty(managers) {
					failures = append(failures, fmt.Errorf("BMC %s: no systems or managers collected", bmcID))
				}
				collectionMu.Unlock()

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
					"ID":                 bmcID,
					"Type":               "",
					"Name":               "",
					"FQDN":               trimmedHost,
					"User":               bmcCreds.Username,
					"MACRequired":        true,
					"RediscoverOnUpdate": false,
					"Systems":            systems,
					"Managers":           managers,
					"SchemaVersion":      1,
				}

				// optionally, add the MACAddr property if we find a matching IP
				// from the correct ethernet interface. trimmedHost was resolved
				// to an IPv4 address above, so hostname-targeted scans perform
				// the lookup by address: net.ParseIP cannot parse a name, and
				// the lookup never matched for any scan target that was not
				// already an IP literal.
				mac, err := FindMACAddressWithIP(config, net.ParseIP(trimmedHost))
				if err != nil {
					log.Warn().Msgf("failed to find MAC address with IP '%s'", trimmedHost)
				}
				if mac != "" {
					data["MACAddr"] = mac
				}

				// add data output to collections
				collectionMu.Lock()
				collection = append(collection, data)
				collectionMu.Unlock()
			}
		}()
	}

	// Queue each active host once. Results are collected concurrently below.
	queued := make(map[string]struct{}, len(*assets))
	for _, ps := range *assets {
		if !ps.State {
			continue
		}
		if _, ok := queued[ps.Host]; ok {
			continue
		}
		queued[ps.Host] = struct{}{}
		chanAssets <- ps
	}

	close(chanAssets)
	wg.Wait()
	if err := errors.Join(failures...); err != nil {
		return collection, err
	}

	var (
		output     []byte
		formatType format.DataFormat
	)

	// format our output to write to file or standard out
	formatType = format.DataFormatFromFileExt(params.OutputPath, params.OutputFormat)
	output, err = format.MarshalData(collection, formatType)
	if err != nil {
		return collection, fmt.Errorf("failed to marshal collect output: %w", err)
	}

	// write data to file in preset directory if output path is set using set format
	if params.OutputDir != "" {
		for _, data := range collection {
			var (
				finalPath = filepath.Join(params.OutputDir, fmt.Sprint(data["ID"]), fmt.Sprintf("%d.%s", time.Now().Unix(), formatType))
				finalDir  = filepath.Dir(finalPath)
			)
			// if it doesn't, make the directory and write file
			if err := os.MkdirAll(finalDir, 0755); err != nil {
				return collection, fmt.Errorf("create collect output directory: %w", err)
			}
			if err := os.WriteFile(finalPath, output, 0644); err != nil {
				return collection, fmt.Errorf("write collect output: %w", err)
			}
		}
	}

	// write data to only to the path set (no preset directory structure)
	if params.OutputPath != "" {
		// if it doesn't, make the directory and write file
		if err := os.MkdirAll(filepath.Dir(params.OutputPath), 0755); err != nil {
			return collection, fmt.Errorf("create collect output directory: %w", err)
		}
		if err := os.WriteFile(params.OutputPath, output, 0644); err != nil {
			return collection, fmt.Errorf("write collect output: %w", err)
		}
	}

	return collection, nil
}

// FindMACAddressWithIP() returns the MAC address of an ethernet interface with
// a matching IPv4Address. Returns an empty string and error if there are no matches
// found.
func FindMACAddressWithIP(config crawler.CrawlerConfig, targetIP net.IP) (string, error) {
	// a nil target IP can never equal an interface address; fail fast
	// instead of connecting to the BMC and comparing every interface
	// against "<nil>" (hostname targets are resolved to an IP before
	// this is called — see CollectInventory — but resolution can fail)
	if targetIP == nil {
		return "", fmt.Errorf("cannot find MAC address: target IP is nil")
	}

	// get the managers to find the BMC MAC address compared with IP
	//
	// NOTE: Since we don't have a RedfishEndpoint type abstraction in
	// magellan and the crawler crawls for systems information, it
	// may just make more sense to get the managers directly via
	// gofish (at least for now). If there's a need for grabbing more
	// manager information in the future, we can move the logic into
	// the crawler.
	client, err := bmc.DefaultManager.Connect(config)
	if err != nil {
		event := log.Error()
		event.Msg("failed to connect to BMC")
		return "", err
	}
	defer client.Logout()

	var (
		rf_service  = client.GetService()
		rf_managers []*schemas.Manager
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
			log.Warn().Msg("failed to get manager Ethernet interfaces for optional MAC lookup")
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
