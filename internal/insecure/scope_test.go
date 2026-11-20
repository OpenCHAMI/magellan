package insecure

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScopeSetValidValues(t *testing.T) {
	tests := []struct {
		input   string
		want    Scope
		wantBMC bool
		wantSec bool
	}{
		{input: "none", want: None, wantBMC: false, wantSec: false},
		{input: "bmc", want: BMC, wantBMC: true, wantSec: false},
		{input: "secrets", want: Secrets, wantBMC: false, wantSec: true},
		{input: "both", want: Both, wantBMC: true, wantSec: true},
		// Aliases preserve the old boolean flag and INSECURE=true-style env vars.
		{input: "true", want: BMC, wantBMC: true, wantSec: false},
		{input: "false", want: None, wantBMC: false, wantSec: false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var s Scope
			require.NoError(t, s.Set(tt.input))
			require.Equal(t, tt.want, s)
			require.Equal(t, tt.wantBMC, s.BMC())
			require.Equal(t, tt.wantSec, s.Secrets())
			require.Equal(t, string(tt.want), s.String())
			require.Equal(t, "Scope", s.Type())
		})
	}
}

func TestScopeSetInvalidValues(t *testing.T) {
	for _, input := range []string{"", "yes", "BMC", "all", "0"} {
		t.Run(input, func(t *testing.T) {
			s := Both
			require.ErrorContains(t, s.Set(input), "invalid insecure scope")
			// Invalid values must not clobber the current scope.
			require.Equal(t, Both, s)
		})
	}
}

func TestScopeHelpMapCoversAllValues(t *testing.T) {
	for _, s := range []Scope{None, BMC, Secrets, Both} {
		require.Contains(t, ScopeHelpMap, string(s))
	}
	require.Len(t, ScopeHelpMap, 4)
}
