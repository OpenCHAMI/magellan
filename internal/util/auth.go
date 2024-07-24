package util

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// LoadAccessToken() tries to load a JWT string from an environment
// variable, file, or config in that order. If loading the token
// fails with one options, it will fallback to the next option until
// all options are exhausted.
//
// Returns a token as a string with no error if successful.
// Alternatively, returns an empty string with an error if a token is
// not able to be loaded.
func LoadAccessToken(path string) (string, error) {
	// try to load token from env var
	testToken := os.Getenv("ACCESS_TOKEN")
	if testToken != "" {
		return testToken, nil
	}

	// try reading access token from a file
	b, err := os.ReadFile(path)
	if err == nil {
		return string(b), nil
	}

	// TODO: try to load token from config
	testToken = viper.GetString("access_token")
	if testToken != "" {
		return testToken, nil
	}
	return "", fmt.Errorf("failed toload token from environment variable, file, or config")
}
