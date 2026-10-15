package crawler

import (
	"testing"

	"github.com/openchami/magellan/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestMergeAndExtractValues(t *testing.T) {
	a, b := 1, 2
	values := extractPtrMapValues(map[string]*int{"a": &a, "b": &b})
	require.ElementsMatch(t, []int{1, 2}, values)
	old := &models.InventoryDetail{URI: "/old"}
	merged := merge(map[string]*models.InventoryDetail{"/old": old}, []models.InventoryDetail{{URI: "/new"}})
	require.Same(t, old, merged["/old"])
	require.Equal(t, "/new", merged["/new"].URI)
}