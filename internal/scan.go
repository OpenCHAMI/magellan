package magellan

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/bikeshack/magellan/internal/util"
)

type ScannedResult struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	State    bool   `json:"state"`
}

func rawConnect(host string, ports []int, timeout int, keepOpenOnly bool) []ScannedResult {
	results := []ScannedResult{}
	for _, p := range ports {
		result := ScannedResult{
			Host:     host,
			Port:     p,
			Protocol: "tcp",
			State:    false,
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

func GenerateHosts(subnet string, mask string, begin uint8, end uint8) []string {
	hosts := []string{}
	ip := net.ParseIP(subnet).To4()
	for i := begin; i < end; i++ {
		ip = util.GetNextIP(ip, 1)
		hosts = append(hosts, fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3]))
	}
	return hosts
}

func GenerateHostsWithCIDR(subnet string) []string {
	// check for network with valid CIDR
	ip, network, err := net.ParseCIDR(subnet)
	if err != nil && (network != nil || ip != nil) {
		network.Mask = ip.DefaultMask()
	}

	// check for IP with no CIDR
	if ip == nil {
		ip = net.ParseIP(subnet)
		if ip == nil {
			return nil
		}
	}

	if network == nil {
		network = &net.IPNet{
			Mask: ip.DefaultMask(),
		}
	}
	
	// get all IP addresses in network
	return generateHosts(ip, network.Mask)
}

func GenerateHostsWithSubnet(subnet string, subnetMask string) []string {
	// if no subnet mask, use a default 24-bit mask (for now)
	if subnetMask != "" {
		ip, network, err := net.ParseCIDR(subnet)
		if err != nil && (network != nil || ip != nil) {
			network.Mask = ip.DefaultMask()
		}
		// check for IP with no CIDR
		if ip == nil {
			ip = net.ParseIP(subnet)
			if ip == nil {
				return nil
			}
		}

		if network == nil {
			network = &net.IPNet{
				Mask: ip.DefaultMask(),
			}
		}
		return generateHosts(ip, network.Mask)
	} else {
		ip := net.ParseIP(subnetMask)
		if ip != nil {
			return []string{}
		}
		return generateHosts(ip, ip.DefaultMask())
	}
}

func generateHosts(ip net.IP, mask net.IPMask) []string {
	// get all IP addresses in network
	ones, _ := mask.Size()
	hosts := []string{}
	fmt.Printf("ones: %d\n", ones)
	for i := 0; i < 32-ones; i++ {
		// ip[3] = byte(i)
		ip = util.GetNextIP(ip, 1)
		hosts = append(hosts, fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3]))
	}
	return hosts
}

func ScanForAssets(hosts []string, ports []int, threads int, timeout int, disableProbing bool) []ScannedResult {
	results := make([]ScannedResult, 0, len(hosts))
	done := make(chan struct{}, threads+1)
	chanHost := make(chan string, threads+1)
	// chanPort := make(chan int, threads+1)

	var wg sync.WaitGroup
	wg.Add(threads)
	for i := 0; i < threads; i++ {
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
						res, _, err := util.MakeRequest(url, "GET", nil, nil)
						if err != nil || res == nil {
							continue
						} else if res.StatusCode != http.StatusOK {
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

func GetDefaultPorts() []int {
	return []int{HTTPS_PORT, IPMI_PORT}
}
