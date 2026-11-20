// Package insecure implements the scoped --insecure flag value. The flag
// selects which connections skip TLS certificate verification: none, bmc,
// secrets, or both. It replaces the old boolean --insecure flag, which mapped
// to the bmc scope; the bare form (--insecure/-i) and the true/false aliases
// preserve the previous behavior.
package insecure

import "fmt"

// Scope selects which connections skip TLS certificate verification.
type Scope string

// Supported insecure scopes.
const (
	None    Scope = "none"
	BMC     Scope = "bmc"
	Secrets Scope = "secrets"
	Both    Scope = "both"
)

// ScopeHelpMap maps scope values to one-line descriptions. It drives the
// --insecure flag shell completion.
var ScopeHelpMap = map[string]string{
	string(None):    "Verify TLS certificates everywhere (default)",
	string(BMC):     "Skip TLS verification for BMC connections",
	string(Secrets): "Skip TLS verification for the secret manager connection",
	string(Both):    "Skip TLS verification for BMC and secret manager connections",
}

// Set validates v and stores it as the scope. It implements pflag.Value and
// accepts "true"/"false" as aliases for "bmc"/"none" so existing
// --insecure=true|false invocations and INSECURE=true-style environment
// variables keep working.
func (s *Scope) Set(v string) error {
	switch v {
	case "true":
		*s = BMC
	case "false":
		*s = None
	default:
		switch Scope(v) {
		case None, BMC, Secrets, Both:
			*s = Scope(v)
		default:
			return fmt.Errorf("invalid insecure scope %q (valid: none, bmc, secrets, both)", v)
		}
	}
	return nil
}

// String returns the current scope as a string. It implements pflag.Value.
func (s Scope) String() string {
	return string(s)
}

// Type returns the pflag value type name. It implements pflag.Value.
func (s Scope) Type() string {
	return "Scope"
}

// BMC reports whether TLS verification is skipped for BMC connections.
func (s Scope) BMC() bool {
	return s == BMC || s == Both
}

// Secrets reports whether TLS verification is skipped for the secret manager
// connection.
func (s Scope) Secrets() bool {
	return s == Secrets || s == Both
}
