package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/OpenCHAMI/magellan/pkg/crawler"
	"github.com/spf13/cobra"
)

var crawlCmd = &cobra.Command{
	Use:   "crawl",
	Short: "Crawl a single BMC for inventory information",
	Run: func(cmd *cobra.Command, args []string) {
		systems, err := crawler.CrawlBMC(crawler.CrawlerConfig{
			URI:      cmd.Flag("uri").Value.String(),
			Username: cmd.Flag("username").Value.String(),
			Password: cmd.Flag("password").Value.String(),
			Insecure: cmd.Flag("insecure").Value.String() == "true",
		})
		if err != nil {
			panic(err)
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
	crawlCmd.Flags().StringP("uri", "u", "", "URI of the BMC")
	crawlCmd.Flags().StringP("username", "n", "", "Username for the BMC")
	crawlCmd.Flags().StringP("password", "p", "", "Password for the BMC")
	crawlCmd.Flags().BoolP("insecure", "i", false, "Ignore SSL errors")

	rootCmd.AddCommand(crawlCmd)
}
