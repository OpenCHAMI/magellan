// Package service provides an in-process façade over magellan's BMC operations.
//
// Both the CLI and the (forthcoming) daemon are intended to drive BMC work
// through this single shared core so behavior stays consistent across
// front-ends. It composes the existing magellan, crawler, power, and bmc
// packages; it deliberately holds no logic of its own beyond wiring.
package service

import (
	"context"

	"github.com/OpenCHAMI/magellan/internal/format"
	magellan "github.com/OpenCHAMI/magellan/pkg"
	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/OpenCHAMI/magellan/pkg/crawler"
	"github.com/OpenCHAMI/magellan/pkg/power"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/stmcginnis/gofish/schemas"
)

// Service is the shared BMC interaction core. A single instance is intended to
// be long-lived (owned by the daemon) or short-lived (constructed per CLI
// invocation); it is safe for concurrent use because the underlying Manager is.
type Service struct {
	// Manager is the BMC connection authority. Defaults to bmc.DefaultManager.
	Manager *bmc.Manager
	// Secrets is the credential store used to resolve BMC credentials.
	Secrets secrets.SecretStore
	// Insecure controls whether TLS verification is skipped when connecting.
	Insecure bool
}

// New constructs a Service backed by the process-wide BMC manager and the given
// credential store.
func New(store secrets.SecretStore) *Service {
	return &Service{
		Manager: bmc.DefaultManager,
		Secrets: store,
	}
}

// connConfig builds a connection config for a BMC URI using the service's
// credential store and TLS settings.
func (s *Service) connConfig(uri string) crawler.CrawlerConfig {
	return crawler.CrawlerConfig{
		URI:             uri,
		Insecure:        s.Insecure,
		CredentialStore: s.Secrets,
		UseDefault:      true,
	}
}

// Discover scans the network for BMC (and optionally PDU) assets.
func (s *Service) Discover(params *magellan.ScanParams) []magellan.RemoteAsset {
	return magellan.ScanForAssets(params)
}

// Collect crawls the given assets for Redfish inventory. SecretStore on params
// is filled from the service when unset.
func (s *Service) Collect(assets []magellan.RemoteAsset, params *magellan.CollectParams) ([]map[string]any, error) {
	if params.SecretStore == nil {
		params.SecretStore = s.Secrets
	}
	return magellan.CollectInventory(&assets, params)
}

// Inventory returns the Redfish systems and managers for a single BMC.
func (s *Service) Inventory(uri string) ([]crawler.InventoryDetail, []crawler.Manager, error) {
	cfg := s.connConfig(uri)
	systems, err := crawler.CrawlBMCForSystems(cfg)
	if err != nil {
		return systems, nil, err
	}
	managers, err := crawler.CrawlBMCForManagers(cfg)
	return systems, managers, err
}

// crawlableNode adapts a (BMC URI, system ID) pair to power's node type.
func (s *Service) crawlableNode(uri, systemID string) power.CrawlableNode {
	return power.CrawlableNode{
		ConnConfig: s.connConfig(uri),
		NodeID:     systemID,
	}
}

// PowerState returns the current power state of a ComputerSystem.
func (s *Service) PowerState(ctx context.Context, uri, systemID string) (schemas.PowerState, error) {
	return power.GetPowerState(ctx, s.crawlableNode(uri, systemID))
}

// ResetTypes returns the reset types supported by a ComputerSystem.
func (s *Service) ResetTypes(ctx context.Context, uri, systemID string) ([]schemas.ResetType, error) {
	return power.GetResetTypes(ctx, s.crawlableNode(uri, systemID))
}

// Reset issues a reset of the given raw Redfish type to a ComputerSystem,
// returning the gofish task-monitor handle when the BMC models it asynchronously.
func (s *Service) Reset(ctx context.Context, uri, systemID string, resetType schemas.ResetType) (*schemas.TaskMonitorInfo, error) {
	return power.ResetComputerSystem(ctx, s.crawlableNode(uri, systemID), resetType)
}

// ResetOperation performs a vendor-neutral power Operation (e.g. bmc.OpOff) on a
// ComputerSystem, resolving it to a supported reset type with the
// graceful→forced fallback chain. It returns bmc.ErrUnsupportedOperation when the
// operation cannot be satisfied by the target's advertised reset types.
func (s *Service) ResetOperation(ctx context.Context, uri, systemID string, op bmc.Operation) (*schemas.TaskMonitorInfo, error) {
	return power.ResetOperation(ctx, s.crawlableNode(uri, systemID), op)
}

// PowerTransition performs a vendor-neutral power Operation and confirms it took
// effect (polling power state to its target or following the BMC's async task),
// escalating a timed-out graceful operation to its forced equivalent per opts.
// This is the synchronous primitive the daemon will wrap as a pollable async
// transition.
func (s *Service) PowerTransition(ctx context.Context, uri, systemID string, op bmc.Operation, opts bmc.TransitionOptions) (*bmc.TransitionResult, error) {
	return power.PowerTransition(ctx, s.crawlableNode(uri, systemID), op, opts)

}

// Close releases any cached BMC sessions held by the manager.
func (s *Service) Close() {
	s.Manager.LogoutAll()
}

// Format is re-exported for callers that need to reference data formats without
// importing the internal package directly.
type Format = format.DataFormat
