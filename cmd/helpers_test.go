package cmd

import (
	"errors"
	"testing"

	"github.com/openchami/magellan/pkg/pdu"
	"github.com/stretchr/testify/require"
)

func TestIsValidCredsJSON(t *testing.T) {
	require.True(t, isValidCredsJSON(`{"username":"user","password":"pass"}`))
	require.False(t, isValidCredsJSON(`{"username":"user"}`))
	require.False(t, isValidCredsJSON(`{`))
}

func TestTransformToSMDFormat(t *testing.T) {
	got := transformToSMDFormat(&pdu.PDUInventory{
		Hostname: "x3000m0",
		Outlets: []pdu.PDUOutlet{
			{ID: "35", Name: "first", PowerState: "ON", SocketType: "C13"},
			{ID: "BA35", Name: "second", PowerState: "OFF"},
			{ID: "invalid"},
		},
	})
	require.Len(t, got, 1)
	require.Equal(t, "x3000m0", got[0]["ID"])
	inv := got[0]["PDUInventory"].(map[string]any)
	outlets := inv["Outlets"].([]map[string]any)
	require.Len(t, outlets, 2)
	require.Equal(t, "p0v35", outlets[0]["id_suffix"])
	require.Equal(t, "p1v35", outlets[1]["id_suffix"])
}

func TestRootHelpers(t *testing.T) {
	checkBindFlagError(nil)
	checkBindFlagError(errors.New("expected"))
	checkRegisterFlagCompletionError(nil)
	checkRegisterFlagCompletionError(errors.New("expected"))
	got := helpMapToSlice(map[string]string{"json": "JSON format"})
	require.Equal(t, []string{"json\tJSON format"}, got)
	values, directive := completionFormatData(nil, nil, "")
	require.Len(t, values, 3)
	require.Equal(t, 0, int(directive))
}
