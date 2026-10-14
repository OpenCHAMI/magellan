package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/openchami/magellan/internal/format"
	magellan "github.com/openchami/magellan/pkg"
	"github.com/openchami/magellan/pkg/bmc"
	"github.com/stmcginnis/gofish/schemas"
)

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// handleInventory crawls one or more BMCs for Redfish inventory and returns the
// same collect-format JSON array that `magellan collect` emits, so the response
// can be piped directly into `magellan send`.
//
// Request body (any of the following):
//
//	{"bmc": "https://bmc1"}                  // a single BMC
//	{"bmcs": ["https://bmc1", "https://bmc2"]} // multiple BMCs
//	["https://bmc1", "https://bmc2"]          // a bare JSON array of BMC URLs
func (s *Server) handleInventory(w http.ResponseWriter, r *http.Request) {
	// Read the raw body so we can accept either a bare JSON array of BMC URLs
	// or an object with "bmc"/"bmcs" fields.
	raw, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read request body: "+err.Error())
		return
	}
	bmcURIs, err := parseInventoryTargets(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(bmcURIs) == 0 {
		writeError(w, http.StatusBadRequest, "at least one BMC is required (field 'bmc', 'bmcs', or a JSON array of URLs)")
		return
	}

	// Build the asset list. Each BMC URL is parsed into the Host/Port split that
	// the collector expects.
	assets := make([]magellan.RemoteAsset, 0, len(bmcURIs))
	for _, u := range bmcURIs {
		asset, err := bmcToAsset(u)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		assets = append(assets, asset)
	}

	insecure := s.svc.Insecure || isInsecureRequest(r)
	concurrency := len(assets)
	if concurrency < 1 {
		concurrency = 1
	}
	params := &magellan.CollectParams{
		Concurrency:  concurrency,
		Insecure:     insecure,
		SecretStore:  s.svc.Secrets,
		OutputFormat: format.FORMAT_JSON,
	}
	collection, err := s.svc.Collect(assets, params)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if collection == nil {
		collection = []map[string]any{}
	}
	// The collect-format array is written directly (not wrapped) so it matches
	// what `magellan send` accepts.
	writeJSON(w, http.StatusOK, collection)
}

// readBody reads the entire request body.
func readBody(r *http.Request) ([]byte, error) {
	defer func() { _ = r.Body.Close() }()
	return io.ReadAll(r.Body)
}

// parseInventoryTargets accepts either a bare JSON array of BMC URL strings or
// an object with "bmc" (single) and/or "bmcs" (array) fields, and returns the
// normalized list of BMC URLs.
func parseInventoryTargets(raw []byte) ([]string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, nil
	}

	// Bare JSON array of strings.
	if strings.HasPrefix(trimmed, "[") {
		var urls []string
		if err := json.Unmarshal([]byte(trimmed), &urls); err != nil {
			return nil, fmt.Errorf("invalid JSON array of BMC URLs: %v", err)
		}
		return urls, nil
	}

	// Object form.
	var obj struct {
		BMC  string   `json:"bmc"`
		BMCs []string `json:"bmcs"`
	}
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return nil, fmt.Errorf("invalid JSON body: %v", err)
	}
	urls := append([]string{}, obj.BMCs...)
	if obj.BMC != "" {
		urls = append([]string{obj.BMC}, urls...)
	}
	return urls, nil
}

// bmcToAsset parses a BMC base URL into the RemoteAsset shape the collector
// expects (Host holds scheme://host, Port holds the port).
func bmcToAsset(raw string) (magellan.RemoteAsset, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return magellan.RemoteAsset{}, fmt.Errorf("invalid BMC URL %q: %v", raw, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return magellan.RemoteAsset{}, fmt.Errorf("invalid BMC URL %q: must include scheme and host", raw)
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return magellan.RemoteAsset{}, fmt.Errorf("invalid port in BMC URL %q: %v", raw, err)
	}
	return magellan.RemoteAsset{
		Host:        fmt.Sprintf("%s://%s", u.Scheme, u.Hostname()),
		Port:        portNum,
		Protocol:    "tcp",
		State:       true,
		Timestamp:   time.Now(),
		ServiceType: magellan.BMC,
	}, nil
}

// handlePowerState returns the power state of a ComputerSystem.
// Query: ?bmc=<url>&system=<id>.
func (s *Server) handlePowerState(w http.ResponseWriter, r *http.Request) {
	bmcURI, systemID, ok := powerTarget(w, r)
	if !ok {
		return
	}
	insecure := s.svc.Insecure || isInsecureRequest(r)
	state, err := s.svc.PowerState(r.Context(), bmcURI, systemID, insecure)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bmc":        bmcURI,
		"system":     systemID,
		"powerState": state,
	})
}

// handleResetTypes returns the reset types a ComputerSystem advertises.
// Query: ?bmc=<url>&system=<id>.
func (s *Server) handleResetTypes(w http.ResponseWriter, r *http.Request) {
	bmcURI, systemID, ok := powerTarget(w, r)
	if !ok {
		return
	}
	insecure := s.svc.Insecure || isInsecureRequest(r)
	types, err := s.svc.ResetTypes(r.Context(), bmcURI, systemID, insecure)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bmc":        bmcURI,
		"system":     systemID,
		"resetTypes": types,
	})
}

// powerActionRequest is the body for POST /v1/power.
type powerActionRequest struct {
	BMC    string `json:"bmc"`
	System string `json:"system"`
	// Exactly one of Operation (vendor-neutral, with fallback/escalation) or
	// ResetType (raw Redfish token) must be set.
	Operation string `json:"operation,omitempty"`
	ResetType string `json:"resetType,omitempty"`
	// Wait confirms the operation reached its target power state (operations
	// only; not valid with a raw ResetType).
	Wait           bool `json:"wait,omitempty"`
	TimeoutSeconds int  `json:"timeoutSeconds,omitempty"`
}

// handlePowerAction issues a power operation or raw reset, optionally confirming
// the resulting power state.
func (s *Server) handlePowerAction(w http.ResponseWriter, r *http.Request) {
	var req powerActionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.BMC == "" || req.System == "" {
		writeError(w, http.StatusBadRequest, "fields 'bmc' and 'system' are required")
		return
	}
	if (req.Operation == "") == (req.ResetType == "") {
		writeError(w, http.StatusBadRequest, "exactly one of 'operation' or 'resetType' is required")
		return
	}
	ctx := r.Context()

	insecure := s.svc.Insecure || isInsecureRequest(r)
	// Raw reset type: passed through unresolved; confirmation is not supported
	// because a raw reset has no vendor-neutral target state.
	if req.ResetType != "" {
		if req.Wait {
			writeError(w, http.StatusBadRequest, "'wait' is only supported with 'operation', not raw 'resetType'")
			return
		}
		if _, err := s.svc.Reset(ctx, req.BMC, req.System, schemas.ResetType(req.ResetType), insecure); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"issued": true, "resetType": req.ResetType})
		return
	}

	// Vendor-neutral operation.
	op := bmc.Operation(req.Operation)
	if !bmc.KnownOperation(op) {
		writeError(w, http.StatusBadRequest, "unknown operation; known: "+operationsList())
		return
	}

	if req.Wait {
		opts := bmc.DefaultTransitionOptions()
		if req.TimeoutSeconds > 0 {
			opts.Timeout = time.Duration(req.TimeoutSeconds) * time.Second
		}
		res, err := s.svc.PowerTransition(ctx, req.BMC, req.System, op, opts, insecure)
		if err != nil {
			writePowerError(w, err)
			return
		}
		status := http.StatusOK
		if !res.Confirmed() {
			// Issued but not confirmed (timed-out / unconfirmable): 202 conveys
			// "accepted, outcome not (yet) confirmed".
			status = http.StatusAccepted
		}
		writeJSON(w, status, map[string]any{
			"operation":   res.Operation,
			"status":      res.Status,
			"finalState":  res.FinalState,
			"escalated":   res.Escalated,
			"escalatedTo": res.EscalatedTo,
		})
		return
	}

	if _, err := s.svc.ResetOperation(ctx, req.BMC, req.System, op, insecure); err != nil {
		writePowerError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"issued": true, "operation": req.Operation})
}

// powerTarget extracts and validates the bmc/system query parameters shared by
// the power read endpoints.
func powerTarget(w http.ResponseWriter, r *http.Request) (bmcURI, systemID string, ok bool) {
	bmcURI = r.URL.Query().Get("bmc")
	systemID = r.URL.Query().Get("system")
	if bmcURI == "" || systemID == "" {
		writeError(w, http.StatusBadRequest, "query parameters 'bmc' and 'system' are required")
		return "", "", false
	}
	return bmcURI, systemID, true
}

// isInsecureRequest checks for insecure override in query param or header.
// Query: ?insecure=true (case-insensitive). Header: X-Magellan-Insecure with
// truthy values (true, 1, t, yes, on), case-insensitive.
func isInsecureRequest(r *http.Request) bool {
	q := r.URL.Query().Get("insecure")
	if truthy(q) {
		return true
	}
	if h := r.Header.Get("X-Magellan-Insecure"); truthy(h) {
		return true
	}
	return false
}

func truthy(v string) bool {
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "true", "1", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}

// writePowerError maps service-layer power errors to HTTP status codes,
// distinguishing an unsupported operation (a client/target mismatch) from a
// generic BMC failure.
func writePowerError(w http.ResponseWriter, err error) {
	var unsupported *bmc.ErrUnsupportedOperation
	if errors.As(err, &unsupported) {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeError(w, http.StatusBadGateway, err.Error())
}

func operationsList() string {
	ops := bmc.Operations()
	parts := make([]string, len(ops))
	for i, op := range ops {
		parts[i] = string(op)
	}
	return joinComma(parts)
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

// decodeJSON decodes a JSON request body, writing a 400 on failure. It rejects
// unknown fields so malformed requests fail loudly rather than silently.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}
