package format

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDataFormat(t *testing.T) {
	var got DataFormat
	for _, value := range []DataFormat{FORMAT_JSON, FORMAT_YAML, FORMAT_LIST} {
		require.NoError(t, got.Set(value.String()))
		require.Equal(t, value, got)
		require.Equal(t, "DataFormat", got.Type())
	}
	require.Error(t, got.Set("toml"))
}

func TestMarshalAndUnmarshalData(t *testing.T) {
	want := map[string]any{"name": "node", "count": 2}
	for _, dataFormat := range []DataFormat{FORMAT_JSON, FORMAT_YAML} {
		t.Run(dataFormat.String(), func(t *testing.T) {
			data, err := MarshalData(want, dataFormat)
			require.NoError(t, err)
			var got map[string]any
			require.NoError(t, UnmarshalData(data, &got, dataFormat))
			require.Equal(t, "node", got["name"])
		})
	}

	_, err := MarshalData(nil, FORMAT_LIST)
	require.Error(t, err)
	_, err = MarshalData(nil, DataFormat("unknown"))
	require.Error(t, err)
	_, err = MarshalData(math.Inf(1), FORMAT_JSON)
	require.Error(t, err)
	require.Error(t, UnmarshalData([]byte("{"), &want, FORMAT_JSON))
	require.Error(t, UnmarshalData([]byte("["), &want, FORMAT_YAML))
	require.Error(t, UnmarshalData(nil, &want, FORMAT_LIST))
	require.Error(t, UnmarshalData(nil, &want, DataFormat("unknown")))
}

func TestDataFormatFromFileExt(t *testing.T) {
	for _, path := range []string{"a.json", "a.JSON"} {
		require.Equal(t, FORMAT_JSON, DataFormatFromFileExt(path, FORMAT_LIST))
	}
	for _, path := range []string{"a.yaml", "a.yml", "a.YAML", "a.YML"} {
		require.Equal(t, FORMAT_YAML, DataFormatFromFileExt(path, FORMAT_LIST))
	}
	require.Equal(t, FORMAT_LIST, DataFormatFromFileExt("a.txt", FORMAT_LIST))
}
