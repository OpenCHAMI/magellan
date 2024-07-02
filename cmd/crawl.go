package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/OpenCHAMI/magellan/pkg/crawler"
	"github.com/spf13/cobra"
)

var crawlCmd = &cobra.Command{
	Use: "crawl [uri]",
	Long: "Crawl a single BMC for inventory information\n" +
		"\n" +
		"Example:\n" +
		"  magellan crawl https://bmc.example.com",
	Short: "Crawl a single BMC for inventory information",
	Args: func(cmd *cobra.Command, args []string) error {
		// Validate that the only argument is a valid URI
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			return err
		}
		parsedURI, err := url.ParseRequestURI(args[0])
		if err != nil {
			return fmt.Errorf("invalid URI specified: %s", args[0])
		}
		// Remove any trailing slashes
		parsedURI.Path = strings.TrimSuffix(parsedURI.Path, "/")
		// Collapse any doubled slashes
		parsedURI.Path = strings.ReplaceAll(parsedURI.Path, "//", "/")
		// Update the URI in the args slice
		args[0] = parsedURI.String()
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		systems, err := crawler.CrawlBMC(crawler.CrawlerConfig{
			URI:      args[0],
			Username: cmd.Flag("username").Value.String(),
			Password: cmd.Flag("password").Value.String(),
			Insecure: cmd.Flag("insecure").Value.String() == "true",
		})
		if err != nil {
			log.Fatalf("Error crawling BMC: %v", err)
		}
		// Marshal the inventory details to JSON
		jsonData, err := json.MarshalIndent(systems, "", "  ")
		if err != nil {
			fmt.Println("Error marshalling to JSON:", err)
			return
		}

		// Print the pretty JSON
		fmt.Println(string(jsonData))
	},
}

func init() {
	crawlCmd.Flags().StringP("username", "u", "", "Username for the BMC")
	crawlCmd.Flags().StringP("password", "p", "", "Password for the BMC")
	crawlCmd.Flags().BoolP("insecure", "i", false, "Ignore SSL errors")

	rootCmd.AddCommand(crawlCmd)
}
