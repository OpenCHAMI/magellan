package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/OpenCHAMI/magellan/internal/server"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/OpenCHAMI/magellan/pkg/service"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	serveHost      string
	servePort      int
	serveTLSCert   string
	serveTLSKey    string
	serveAuthToken string
)

// ServeCmd runs magellan as a long-lived REST service over the shared BMC core,
// so other OpenCHAMI tools can delegate discovery, inventory, and power
// operations to magellan instead of talking to BMCs directly (RFD #133).
var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run magellan as a long-lived BMC service (REST API)",
	Long: "Run magellan as a persistent daemon exposing a REST API for BMC inventory and\n" +
		"power operations, backed by the same shared core the CLI uses. The server runs\n" +
		"until it receives SIGINT or SIGTERM, then drains in-flight requests.",
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve BMC credentials from the local secret store, matching the
		// other subcommands. Per-request credentials are a later enhancement.
		store, err := secrets.OpenStore(secretsFile)
		if err != nil {
			log.Warn().Err(err).Str("path", secretsFile).Msg("failed to open secrets store; BMC operations will fail until credentials are available")
		}

		svc := service.New(store)
		svc.Insecure = insecure
		defer svc.Close()

		srv := server.New(svc, server.Config{
			Addr:      fmt.Sprintf("%s:%d", serveHost, servePort),
			TLSCert:   serveTLSCert,
			TLSKey:    serveTLSKey,
			AuthToken: serveAuthToken,
		})

		// Cancel the run context on SIGINT/SIGTERM to trigger graceful shutdown.
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		if err := srv.ListenAndServe(ctx); err != nil {
			log.Fatal().Err(err).Msg("magellan daemon error")
		}
		log.Info().Msg("magellan daemon stopped")
	},
}

func init() {
	ServeCmd.Flags().StringVar(&serveHost, "host", "", "Host/IP to bind (default: all interfaces)")
	ServeCmd.Flags().IntVar(&servePort, "port", 8443, "Port to listen on")
	ServeCmd.Flags().StringVar(&serveTLSCert, "tls-cert", "", "Path to TLS certificate (enables HTTPS when set with --tls-key)")
	ServeCmd.Flags().StringVar(&serveTLSKey, "tls-key", "", "Path to TLS private key")
	ServeCmd.Flags().StringVar(&serveAuthToken, "auth-token", "", "Require this bearer token on /v1 routes (auth disabled when empty)")
	ServeCmd.Flags().StringVar(&secretsFile, "secrets-file", "", "Path to the node secrets file")
	ServeCmd.Flags().BoolVarP(&insecure, "insecure", "i", false, "Ignore BMC TLS verification errors")

	checkBindFlagError(viper.BindPFlag("server.host", ServeCmd.Flags().Lookup("host")))
	checkBindFlagError(viper.BindPFlag("server.port", ServeCmd.Flags().Lookup("port")))

	rootCmd.AddCommand(ServeCmd)
}
