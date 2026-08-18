package util

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIPAddrStrToInt(t *testing.T) {
	got, err := IPAddrStrToInt("1.2.3.4")
	require.NoError(t, err)
	require.Equal(t, 0x01020304, got)
	_, err = IPAddrStrToInt("invalid")
	require.Error(t, err)
}

func TestIsEmpty(t *testing.T) {
	require.True(t, IsEmpty([]int{}))
	require.False(t, IsEmpty([]int{1}))
}

func TestErrors(t *testing.T) {
	err := FormatErrorList([]error{errors.New("first"), errors.New("second")})
	require.Contains(t, err.Error(), "first")
	require.Contains(t, err.Error(), "second")
	require.True(t, HasErrors([]error{err}))
	require.False(t, HasErrors(nil))
}

func TestCheckUntil(t *testing.T) {
	calls := 0
	require.NoError(t, CheckUntil(time.Millisecond, time.Second, func() (bool, error) {
		calls++
		return calls == 2, nil
	}))
	wantErr := errors.New("stop")
	require.ErrorIs(t, CheckUntil(time.Millisecond, time.Second, func() (bool, error) {
		return false, wantErr
	}), wantErr)
	require.Error(t, CheckUntil(time.Millisecond, 3*time.Millisecond, func() (bool, error) {
		return false, nil
	}))
}

func TestPathHelpers(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	d, name, ext := SplitPathForViper(file)
	require.Equal(t, dir, d)
	require.Equal(t, "config", name)
	require.Equal(t, "yaml", ext)
	_, exists := PathExists(dir)
	require.True(t, exists)
	_, exists = PathExists(filepath.Join(dir, "missing"))
	require.False(t, exists)

	created, err := MakeOutputDirectory(dir, false)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(created, dir))
	_, err = MakeOutputDirectory(dir, false)
	require.Error(t, err)
	createdAgain, err := MakeOutputDirectory(dir, true)
	require.NoError(t, err)
	require.Equal(t, created, createdAgain)
}
