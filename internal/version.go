package magellan

import (
	"fmt"
	"os/exec"
	"strings"
)

// VersionCommit() returns a string with 'r{commit_count}.{commit_version}' format.
func VersionCommit() string {
	var (
		version  string
		revlist  []byte
		revparse []byte
		err      error
	)
	revlist, err = exec.Command("git", "rev-list", "--count", "HEAD").Output()
	if err != nil {
		return ""
	}

	revparse, err = exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	version = fmt.Sprintf("r%s.%s", strings.TrimRight(string(revlist), "\n"), string(revparse))
	return strings.TrimRight(version, "\n")
}

// VersionTag() returns  a string with format '{git_tag}' using the `git describe` command.
func VersionTag() string {
	var (
		describe []byte
		err      error
	)
	describe, err = exec.Command("git", "describe", "--long", "HEAD").Output()
	if err != nil {
		return ""
	}

	return strings.TrimRight(string(describe), "\n")
}
