package url

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

func Sanitize(uri string) (string, error) {
	// URL sanitanization for host argument
	parsedURI, err := url.ParseRequestURI(uri)
	if err != nil {
		return "", fmt.Errorf("failed to parse URI: %w", err)
	}
	// Collapse any doubled slashes
	parsedURI.Path = strings.ReplaceAll(parsedURI.Path, "//", "/")
	// Remove any trailing slashes
	parsedURI.Path = strings.TrimRight(parsedURI.Path, "/")
	return parsedURI.String(), nil
}

func TrimScheme(uri string) string {
	const prefix = "https://"
	if strings.Contains(uri, prefix) {
		return strings.TrimPrefix(uri, prefix)
	}
	return uri
}

// FormatHosts() takes a list of hosts and ports and builds full URLs in the
// form of scheme://host:port. If no scheme is provided, it will use "https" by
// default.
//
// Returns a 2D string slice where each slice contains URL host strings for each
// port. The intention is to have all of the URLs for a single host combined into
// a single slice to initiate one goroutine per host, but making request to multiple
// ports.
func FormatHosts(hosts []string, ports []int, scheme string) [][]string {
	// format each positional arg as a complete URL
	var formattedHosts [][]string
	for _, host := range hosts {
		if !strings.Contains(host, "://") {
			if scheme == "" {
				scheme = "https"
			}
			host = scheme + "://" + host
		}
		uri, err := url.ParseRequestURI(host)
		if err != nil {
			log.Warn().Msgf("invalid URI parsed: %s", host)
			continue
		}

		// tidy up slashes and update arg with new value
		uri.Path = strings.TrimSuffix(uri.Path, "/")
		uri.Path = strings.ReplaceAll(uri.Path, "//", "/")

		// for hosts with unspecified ports, add ports to scan from flag
		if uri.Port() == "" {
			var tmp []string
			for _, port := range ports {
				portURI := *uri
				portURI.Host = net.JoinHostPort(uri.Hostname(), strconv.Itoa(port))
				tmp = append(tmp, portURI.String())
			}
			formattedHosts = append(formattedHosts, tmp)
		} else {
			formattedHosts = append(formattedHosts, []string{uri.String()})
		}

	}
	return formattedHosts
}

// FormatIPs() takes a list of IP addresses and ports and builds full URLs in the
// form of scheme://host:port. If no scheme is provided, it will use "https" by
// default.
//
// Returns a 2D string slice where each slice contains URL host strings for each
// port. The intention is to have all of the URLs for a single host combined into
// a single slice to initiate one goroutine per host, but making request to multiple
// ports.
func FormatIPs(ips []string, ports []int, scheme string, verbose bool) [][]string {
	// format each positional arg as a complete URL
	var formattedHosts [][]string
	for _, ip := range ips {
		if scheme == "" {
			scheme = "https"
		}
		// make an entirely new object since we're expecting just IPs
		if len(ports) == 0 {
			ports = []int{443}
		}
		var tmp []string
		for _, port := range ports {
			uri := &url.URL{Scheme: scheme, Host: net.JoinHostPort(ip, strconv.Itoa(port))}
			tmp = append(tmp, uri.String())
		}
		formattedHosts = append(formattedHosts, tmp)

	}
	return formattedHosts
}
