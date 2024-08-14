package url

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

func Sanitize(uri string) (string, error) {
	// URL sanitanization for host argument
	parsedURI, err := url.ParseRequestURI(uri)
	if err != nil {
		return "", fmt.Errorf("failed to parse URI: %w", err)
	}
	// Remove any trailing slashes
	parsedURI.Path = strings.TrimSuffix(parsedURI.Path, "/")
	// Collapse any doubled slashes
	parsedURI.Path = strings.ReplaceAll(parsedURI.Path, "//", "/")
	return parsedURI.String(), nil
}

// FormatHosts() takes a list of hosts and ports and builds full URLs in the
// form of scheme://host:port. If no scheme is provided, it will use "https" by
// default.
//
// Returns a 2D string slice where each slice contains URL host strings for each
// port. The intention is to have all of the URLs for a single host combined into
// a single slice to initiate one goroutine per host, but making request to multiple
// ports.
func FormatHosts(hosts []string, ports []int, scheme string, verbose bool) [][]string {
	// format each positional arg as a complete URL
	var formattedHosts [][]string
	for _, host := range hosts {
		uri, err := url.ParseRequestURI(host)
		if err != nil {
			if verbose {
				log.Warn().Msgf("invalid URI parsed: %s", host)
			}
			continue
		}

		// check if scheme is set, if not set it with flag or default value ('https' if flag is not set)
		if uri.Scheme == "" {
			if scheme != "" {
				uri.Scheme = scheme
			} else {
				// hardcoded assumption
				uri.Scheme = "https"
			}
		}

		// tidy up slashes and update arg with new value
		uri.Path = strings.TrimSuffix(uri.Path, "/")
		uri.Path = strings.ReplaceAll(uri.Path, "//", "/")

		// for hosts with unspecified ports, add ports to scan from flag
		if uri.Port() == "" {
			var tmp []string
			for _, port := range ports {
				uri.Host += fmt.Sprintf(":%d", port)
				tmp = append(tmp, uri.String())
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
		uri := &url.URL{
			Scheme: scheme,
			Host:   ip,
		}

		// tidy up slashes and update arg with new value
		uri.Path = strings.ReplaceAll(uri.Path, "//", "/")
		uri.Path = strings.TrimSuffix(uri.Path, "/")

		// for hosts with unspecified ports, add ports to scan from flag
		if uri.Port() == "" {
			if len(ports) == 0 {
				ports = append(ports, 443)
			}
			var tmp []string
			for _, port := range ports {
				uri.Host += fmt.Sprintf(":%d", port)
				tmp = append(tmp, uri.String())
			}
			formattedHosts = append(formattedHosts, tmp)
		} else {
			formattedHosts = append(formattedHosts, []string{uri.String()})
		}

	}
	return formattedHosts
}
