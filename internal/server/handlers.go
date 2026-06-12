package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/stmcginnis/gofish/schemas"
)

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// handleInventory crawls a single BMC for its systems and managers.
// Request body: {"bmc": "https://..."}.
func (s *Server) handleInventory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BMC string `json:"bmc"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.BMC == "" {
		writeError(w, http.StatusBadRequest, "field 'bmc' is required")
		return
	}
	systems, managers, err := s.svc.Inventory(req.BMC)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bmc":      req.BMC,
		"systems":  systems,
		"managers": managers,
	})
}

// handlePowerState returns the power state of a ComputerSystem.
// Query: ?bmc=<url>&system=<id>.
func (s *Server) handlePowerState(w http.ResponseWriter, r *http.Request) {
	bmcURI, systemID, ok := powerTarget(w, r)
	if !ok {
		return
	}
	state, err := s.svc.PowerState(r.Context(), bmcURI, systemID)
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
	types, err := s.svc.ResetTypes(r.Context(), bmcURI, systemID)
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

	// Raw reset type: passed through unresolved; confirmation is not supported
	// because a raw reset has no vendor-neutral target state.
	if req.ResetType != "" {
		if req.Wait {
			writeError(w, http.StatusBadRequest, "'wait' is only supported with 'operation', not raw 'resetType'")
			return
		}
		if _, err := s.svc.Reset(ctx, req.BMC, req.System, schemas.ResetType(req.ResetType)); err != nil {
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
		res, err := s.svc.PowerTransition(ctx, req.BMC, req.System, op, opts)
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

	if _, err := s.svc.ResetOperation(ctx, req.BMC, req.System, op); err != nil {
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
