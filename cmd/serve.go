package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/openchami/magellan/internal/server"
	"github.com/openchami/magellan/pkg/bmc"
	"github.com/openchami/magellan/pkg/service"
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
	serveVendor    string
)

// ServeCmd runs magellan as a long-lived REST service over the shared BMC core,
// so other OpenCHAMI tools can delegate discovery, inventory, and power
// operations to magellan instead of talking to BMCs directly (RFD #133).
var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run magellan as a long-lived BMC service (REST API)",
	Long: `Run magellan as a persistent daemon exposing a REST API for BMC inventory and
power operations, backed by the same shared core the CLI uses. The server runs
until it receives SIGINT or SIGTERM, then drains in-flight requests.`,
	Example: `  # serve on the default port (8500) using a secrets file
  magellan serve --secrets-file secrets.json

  # serve over HTTPS and require a bearer token
  magellan serve --tls-cert cert.pem --tls-key key.pem --auth-token "$TOKEN"

  # bind to localhost on a custom port, ignoring BMC TLS errors (dev only)
  magellan serve --host 127.0.0.1 --port 8080 --insecure

  # configure with environment variables
  SERVER_PORT=9000 SERVER_AUTH_TOKEN="$TOKEN" magellan serve

  # query the running daemon
  curl -H "Authorization: Bearer $TOKEN" \
    "https://localhost:8500/v1/power?bmc=https://172.16.0.10&system=Node0"`,
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve settings via viper so both flags and SERVER_* env vars work.
		var (
			host      = viper.GetString("server.host")
			port      = viper.GetInt("server.port")
			tlsCert   = viper.GetString("server.tls-cert")
			tlsKey    = viper.GetString("server.tls-key")
			authToken = viper.GetString("server.auth-token")
			vendor    = viper.GetString("server.vendor")
			secFile   = viper.GetString("server.secrets-file")
		)

		// viper unifies the --insecure flag, the server.insecure config key,
		// and the INSECURE env var; resolve the scope here since env-only
		// usage never touches the flag variable. The bmc scope drives BMC
		// connections, the secrets scope drives the credential store.
		if scope := viper.GetString("server.insecure"); scope != "" {
			if err := insecureScope.Set(scope); err != nil {
				log.Fatal().Err(err).Msg("invalid insecure scope")
			}
		}
		skipTLS := insecureScope.BMC()

		// Validate the vendor override before opening the credential store.
		if vendor != "" {
			if _, err := bmc.ParseVendor(vendor); err != nil {
				log.Fatal().Err(err).Msg("invalid --vendor value")
			}
		}

		// Resolve BMC credentials from the configured secret store, matching
		// the other subcommands. Per-request credentials are a later
		// enhancement.
		store, err := openSecretStore(secFile)
		if err != nil {
			log.Warn().Err(err).Str("path", secFile).Msg("could not open credential store; BMC operations will fail until credentials are available")
		}

		svc := service.New(store)
		svc.Insecure = skipTLS
		svc.Vendor = vendor
		defer svc.Close()

		srv := server.New(svc, server.Config{
			Addr:      fmt.Sprintf("%s:%d", host, port),
			TLSCert:   tlsCert,
			TLSKey:    tlsKey,
			AuthToken: authToken,
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
	ServeCmd.Flags().IntVar(&servePort, "port", 8500, "Port to listen on")
	ServeCmd.Flags().StringVar(&serveTLSCert, "tls-cert", "", "Path to TLS certificate (enables HTTPS when set with --tls-key)")
	ServeCmd.Flags().StringVar(&serveTLSKey, "tls-key", "", "Path to TLS private key")
	ServeCmd.Flags().StringVar(&serveAuthToken, "auth-token", "", "Require this bearer token on /v1 routes (auth disabled when empty)")
	ServeCmd.Flags().StringVar(&serveVendor, "vendor", "", "Force a vendor plugin (generic|hpe|dell|supermicro|cray|lenovo) instead of auto-detecting.")
	ServeCmd.Flags().StringVar(&secretsFile, "secrets-file", "", "Path to the node secrets file")
	registerInsecureFlag(ServeCmd, "Skip TLS verification for BMC connections (bmc), the secret manager (secrets), both, or none")

	checkBindFlagError(viper.BindPFlag("server.host", ServeCmd.Flags().Lookup("host")))
	checkBindFlagError(viper.BindPFlag("server.port", ServeCmd.Flags().Lookup("port")))
	checkBindFlagError(viper.BindPFlag("server.tls-cert", ServeCmd.Flags().Lookup("tls-cert")))
	checkBindFlagError(viper.BindPFlag("server.tls-key", ServeCmd.Flags().Lookup("tls-key")))
	checkBindFlagError(viper.BindPFlag("server.auth-token", ServeCmd.Flags().Lookup("auth-token")))
	checkBindFlagError(viper.BindPFlag("server.vendor", ServeCmd.Flags().Lookup("vendor")))
	checkBindFlagError(viper.BindPFlag("server.secrets-file", ServeCmd.Flags().Lookup("secrets-file")))
	checkBindFlagError(viper.BindPFlag("server.insecure", ServeCmd.Flags().Lookup("insecure")))

	// The man page documents both a SERVER_*-prefixed name and a short unprefixed
	// alias for each setting; bind both so either form resolves. AutomaticEnv
	// already honors the SERVER_* forms via SetEnvKeyReplacer in root.go.
	checkBindFlagError(viper.BindEnv("server.host", "SERVER_HOST", "HOST"))
	checkBindFlagError(viper.BindEnv("server.port", "SERVER_PORT", "PORT"))
	checkBindFlagError(viper.BindEnv("server.tls-cert", "SERVER_TLS_CERT", "TLS_CERT"))
	checkBindFlagError(viper.BindEnv("server.tls-key", "SERVER_TLS_KEY", "TLS_KEY"))
	checkBindFlagError(viper.BindEnv("server.auth-token", "SERVER_AUTH_TOKEN", "AUTH_TOKEN"))
	checkBindFlagError(viper.BindEnv("server.vendor", "SERVER_VENDOR"))
	checkBindFlagError(viper.BindEnv("server.secrets-file", "SECRETS_FILE"))
	checkBindFlagError(viper.BindEnv("server.insecure", "INSECURE"))

	rootCmd.AddCommand(ServeCmd)
}
