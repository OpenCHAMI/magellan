package magellan

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/OpenCHAMI/magellan/internal/util"
)

type ScannedResult struct {
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Protocol  string    `json:"protocol"`
	State     bool      `json:"state"`
	Timestamp time.Time `json:"timestamp"`
}

// ScanForAssets() performs a net scan on a network to find available services
// running. The function expects a list of hosts and ports to make requests.
// Note that each all ports will be used per host.
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
func ScanForAssets(hosts []string, ports []int, concurrency int, timeout int, disableProbing bool, verbose bool) []ScannedResult {
	var (
		results  = make([]ScannedResult, 0, len(hosts))
		done     = make(chan struct{}, concurrency+1)
		chanHost = make(chan string, concurrency+1)
	)

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			for {
				host, ok := <-chanHost
				if !ok {
					wg.Done()
					return
				}
				scannedResults := rawConnect(host, ports, timeout, true)
				if !disableProbing {
					probeResults := []ScannedResult{}
					for _, result := range scannedResults {
						url := fmt.Sprintf("https://%s:%d/redfish/v1/", result.Host, result.Port)
						res, _, err := util.MakeRequest(nil, url, "GET", nil, nil)
						if err != nil || res == nil {
							if verbose {
								fmt.Printf("failed to make request: %v\n", err)
							}
							continue
						} else if res.StatusCode != http.StatusOK {
							if verbose {
								fmt.Printf("request returned code: %v\n", res.StatusCode)
							}
							continue
						} else {
							probeResults = append(probeResults, result)
						}
					}
					results = append(results, probeResults...)
				} else {
					results = append(results, scannedResults...)
				}

			}
		}()
	}

	for _, host := range hosts {
		chanHost <- host
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
	close(chanHost)
	wg.Wait()
	close(done)
	return results
}

// GenerateHosts() builds a list of hosts to scan using the "subnet"
// and "subnetMask" arguments passed. The function is capable of
// distinguishing between IP formats: a subnet with just an IP address (172.16.0.0) and
// a subnet with IP address and CIDR (172.16.0.0/24).
//
// NOTE: If a IP address is provided with CIDR, then the "subnetMask"
// parameter will be ignored. If neither is provided, then the default
// subnet mask will be used instead.
func GenerateHosts(subnet string, subnetMask *net.IP) []string {
	if subnet == "" || subnetMask == nil {
		return nil
	}

	// convert subnets from string to net.IP
	subnetIp := net.ParseIP(subnet)
	if subnetIp == nil {
		// try parse CIDR instead
		ip, network, err := net.ParseCIDR(subnet)
		if err != nil {
			return nil
		}
		subnetIp = ip
		if network != nil {
			t := net.IP(network.Mask)
			subnetMask = &t
		}
	}

	mask := net.IPMask(subnetMask.To4())

	// if no subnet mask, use a default 24-bit mask (for now)
	return generateHosts(&subnetIp, &mask)
}

func GetDefaultPorts() []int {
	return []int{HTTPS_PORT}
}

func rawConnect(host string, ports []int, timeout int, keepOpenOnly bool) []ScannedResult {
	results := []ScannedResult{}
	for _, p := range ports {
		result := ScannedResult{
			Host:      host,
			Port:      p,
			Protocol:  "tcp",
			State:     false,
			Timestamp: time.Now(),
		}
		t := time.Second * time.Duration(timeout)
		port := fmt.Sprint(p)
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), t)
		if err != nil {
			result.State = false
			// fmt.Println("Connecting error:", err)
		}
		if conn != nil {
			result.State = true
			defer conn.Close()
			// fmt.Println("Opened", net.JoinHostPort(host, port))
		}
		if keepOpenOnly {
			if result.State {
				results = append(results, result)
			}
		} else {
			results = append(results, result)
		}
	}

	return results
}

func generateHosts(ip *net.IP, mask *net.IPMask) []string {
	// get all IP addresses in network
	ones, _ := mask.Size()
	hosts := []string{}
	end := int(math.Pow(2, float64((32-ones)))) - 1
	for i := 0; i < end; i++ {
		// ip[3] = byte(i)
		ip = util.GetNextIP(ip, 1)
		if ip == nil {
			continue
		}
		// host := fmt.Sprintf("%v.%v.%v.%v", (*ip)[0], (*ip)[1], (*ip)[2], (*ip)[3])
		// fmt.Printf("host: %v\n", ip.String())
		hosts = append(hosts, ip.String())
	}
	return hosts
}
