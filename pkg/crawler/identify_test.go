package crawler

import (
	"testing"

	"github.com/stmcginnis/gofish/schemas"
	"github.com/stretchr/testify/require"
)

func TestIsBMC(t *testing.T) {
	require.False(t, IsBMC(nil))
	for _, managerType := range []schemas.ManagerType{schemas.BMCManagerType, schemas.ManagementControllerManagerType} {
		require.True(t, IsBMC(&schemas.Manager{ManagerType: managerType}))
	}
	require.False(t, IsBMC(&schemas.Manager{ManagerType: schemas.ManagerType("EnclosureManager")}))
}

func TestMergeAndExtractValues(t *testing.T) {
	a, b := 1, 2
	values := extractPtrMapValues(map[string]*int{"a": &a, "b": &b})
	require.ElementsMatch(t, []int{1, 2}, values)
	old := &InventoryDetail{URI: "/old"}
	merged := merge(map[string]*InventoryDetail{"/old": old}, []InventoryDetail{{URI: "/new"}})
	require.Same(t, old, merged["/old"])
	require.Equal(t, "/new", merged["/new"].URI)
}
