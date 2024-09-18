package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strconv"

	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	withAllHosts bool
	withAllPorts bool
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage found assets in cache.",
	Run: func(cmd *cobra.Command, args []string) {
		// show the help for cache and exit
		if len(args) <= 0 {
			cmd.Help()
			os.Exit(0)
		}
	},
}

var cacheRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a host from a scanned cache list.",
	Run: func(cmd *cobra.Command, args []string) {
		assets := []magellan.RemoteAsset{}
		for _, arg := range args {
			var (
				port int
				uri  *url.URL
				err  error
			)
			uri, err = url.ParseRequestURI(arg)
			if err != nil {
				log.Error().Err(err).Msg("failed to parse arg")
			}

			// convert port to its "proper" type
			port, err = strconv.Atoi(uri.Port())
			if err != nil {
				log.Error().Err(err).Msg("failed to convert port to integer type")
			}
			asset := magellan.RemoteAsset{
				Host: fmt.Sprintf("%s://%s", uri.Scheme, uri.Hostname()),
				Port: port,
			}
			fmt.Printf("%s:%d\n", asset.Host, asset.Port)
			assets = append(assets, asset)
		}
		sqlite.DeleteScannedAssets(cachePath, assets...)
	},
}

func init() {
	cacheRemoveCmd.Flags().BoolVar(&withAllHosts, "--all-hosts", false, "Remove all assets with specified hosts")
	cacheRemoveCmd.Flags().BoolVar(&withAllPorts, "--all-ports", false, "Remove all assets with specified ports")
	cacheCmd.AddCommand(cacheRemoveCmd)
	rootCmd.AddCommand(cacheCmd)
}
