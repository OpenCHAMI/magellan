package magellan

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	urlx "github.com/OpenCHAMI/magellan/internal/url"
	"github.com/OpenCHAMI/magellan/pkg/client"
	"github.com/rs/zerolog/log"
)

type RemoteAsset struct {
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Protocol  string    `json:"protocol"`
	State     bool      `json:"state"`
	Timestamp time.Time `json:"timestamp"`
}

// ScanParams is a collection of commom parameters passed to the CLI
type ScanParams struct {
	TargetHosts    [][]string
	Scheme         string
	Protocol       string
	Concurrency    int
	Timeout        int
	DisableProbing bool
	Verbose        bool
	Debug          bool
}

// ScanForAssets() performs a net scan on a network to find available services
// running. The function expects a list of targets (as [][]string) to make requests.
// The 2D list is to permit one goroutine per BMC node when making each request.
//
// This function runs in a goroutine with the "concurrency" flag setting the
// number of concurrent requests. Only one request is made to each BMC node
// at a time, but setting a value greater than 1 with enable the requests
// to be made concurrently.
//
// If the "disableProbing" flag is set, then the function will skip the extra
// HTTP request made to check if the response was from a Redfish service.
// Otherwise, not receiving a 200 OK response code from the HTTP request will
// remove the service from being stored in the list of scanned results.
//
// Returns a list of scanned results to be stored in cache (but isn't doing here).
func ScanForAssets(params *ScanParams) []RemoteAsset {
	var (
		results   = make([]RemoteAsset, 0, len(params.TargetHosts))
		done      = make(chan struct{}, params.Concurrency+1)
		chanHosts = make(chan []string, params.Concurrency+1)
	)

	if params.Verbose {
		log.Info().Any("args", params).Msg("starting scan...")
	}

	var wg sync.WaitGroup
	wg.Add(params.Concurrency)
	for i := 0; i < params.Concurrency; i++ {
		go func() {
			for {
				hosts, ok := <-chanHosts
				if !ok {
					wg.Done()
					return
				}
				for _, host := range hosts {
					foundAssets, err := rawConnect(host, params.Protocol, params.Timeout, true)
					// if we failed to connect, exit from the function
					if err != nil {
						if params.Verbose {
							log.Debug().Err(err).Msgf("failed to connect to host")
						}
						wg.Done()
						return
					}
					if !params.DisableProbing {
						assetsToAdd := []RemoteAsset{}
						for _, foundAsset := range foundAssets {
							url := fmt.Sprintf("%s:%d/redfish/v1/", foundAsset.Host, foundAsset.Port)
							res, _, err := client.MakeRequest(nil, url, http.MethodGet, nil, nil)
							if err != nil || res == nil {
								if params.Verbose {
									log.Printf("failed to make request: %v\n", err)
								}
								continue
							} else if res.StatusCode != http.StatusOK {
								if params.Verbose {
									log.Printf("request returned code: %v\n", res.StatusCode)
								}
								continue
							} else {
								assetsToAdd = append(assetsToAdd, foundAsset)
							}
						}
						results = append(results, assetsToAdd...)
					} else {
						results = append(results, foundAssets...)
					}
				}
			}
		}()
	}

	for _, hosts := range params.TargetHosts {
		chanHosts <- hosts
	}
	go func() {
		select {
		case <-done:
			wg.Done()
			break
		default:
			time.Sleep(1000)
		}
	}()
	close(chanHosts)
	wg.Wait()
	close(done)

	if params.Verbose {
		log.Info().Msg("scan complete")
	}
	return results
}

// GenerateHostsWithSubnet() builds a list of hosts to scan using the "subnet"
// and "subnetMask" arguments passed. The function is capable of
// distinguishing between IP formats: a subnet with just an IP address (172.16.0.0)
// and a subnet with IP address and CIDR (172.16.0.0/24).
//
// NOTE: If a IP address is provided with CIDR, then the "subnetMask"
// parameter will be ignored. If neither is provided, then the default
// subnet mask will be used instead.
func GenerateHostsWithSubnet(subnet string, subnetMask *net.IPMask, additionalPorts []int, defaultScheme string) [][]string {
	if subnet == "" || subnetMask == nil {
		return nil
	}

	// convert subnets from string to net.IP to test if CIDR is included
	subnetIp := net.ParseIP(subnet)
	if subnetIp == nil {
		// not a valid IP so try again with CIDR
		ip, network, err := net.ParseCIDR(subnet)
		if err != nil {
			return nil
		}
		subnetIp = ip
		if network == nil {
			// use the default subnet mask if a valid one is not provided
			network = &net.IPNet{
				IP:   subnetIp,
				Mask: net.IPv4Mask(255, 255, 255, 0),
			}
		}
		subnetMask = &network.Mask
	}

	// generate new IPs from subnet and format to full URL
	subnetIps := generateIPsWithSubnet(&subnetIp, subnetMask)
	return urlx.FormatIPs(subnetIps, additionalPorts, defaultScheme, false)
}

// GetDefaultPorts() returns a list of default ports. The only reason to have
// this function is to add/remove ports without affecting usage.
func GetDefaultPorts() []int {
	return []int{443}
}

// rawConnect() tries to connect to the host using DialTimeout() and waits
// until a response is receive or if the timeout (in seconds) expires. This
// function expects a full URL such as https://my.bmc.host:443/ to make the
// connection.
func rawConnect(address string, protocol string, timeoutSeconds int, keepOpenOnly bool) ([]RemoteAsset, error) {
	uri, err := url.ParseRequestURI(address)
	if err != nil {
		return nil, fmt.Errorf("failed to split host/port: %w", err)
	}

	// convert port to its "proper" type
	port, err := strconv.Atoi(uri.Port())
	if err != nil {
		return nil, fmt.Errorf("failed to convert port to integer type: %w", err)
	}

	var (
		timeoutDuration = time.Second * time.Duration(timeoutSeconds)
		assets          []RemoteAsset
		asset           = RemoteAsset{
			Host:      fmt.Sprintf("%s://%s", uri.Scheme, uri.Hostname()),
			Port:      port,
			Protocol:  protocol,
			State:     false,
			Timestamp: time.Now(),
		}
	)

	// try to conntect to host (expects host in format [10.0.0.0]:443)
	target := fmt.Sprintf("%s:%s", uri.Hostname(), uri.Port())
	conn, err := net.DialTimeout(protocol, target, timeoutDuration)
	if err != nil {
		asset.State = false
		return nil, fmt.Errorf("failed to dial host: %w", err)
	}
	if conn != nil {
		asset.State = true
		defer conn.Close()
	}
	if keepOpenOnly {
		if asset.State {
			assets = append(assets, asset)
		}
	} else {
		assets = append(assets, asset)
	}

	return assets, nil
}

// generateIPsWithSubnet() returns a collection of host IP strings with a
// provided subnet mask.
//
// TODO: add a way for filtering/exclude specific IPs and IP ranges.
func generateIPsWithSubnet(ip *net.IP, mask *net.IPMask) []string {
	// check if subnet IP and mask are valid
	if ip == nil || mask == nil {
		log.Error().Msg("invalid subnet IP or mask (ip == nil or mask == nil)")
		return nil
	}
	// get all IP addresses in network
	ones, bits := mask.Size()
	hosts := []string{}
	end := int(math.Pow(2, float64((bits-ones)))) - 1
	for i := 0; i < end; i++ {
		ip = client.GetNextIP(ip, 1)
		if ip == nil {
			continue
		}

		hosts = append(hosts, ip.String())
	}
	return hosts
}
