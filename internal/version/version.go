package version

import (
	"fmt"
)

// GitCommit stores the latest Git commit hash.
// Set via -ldflags "-X github.com/davidallendj/magellan/internal/version.GitCommit=$(git rev-parse HEAD)"
var GitCommit string

// BuildTime stores the build timestamp in UTC.
// Set via -ldflags "-X github.com/davidallendj/magellan/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var BuildTime string

// Version indicates the version of the binary, such as a release number or semantic version.
// Set via -ldflags "-X github.com/davidallendj/magellan/internal/version.Version=v1.0.0"
var Version string

// GitBranch holds the name of the Git branch from which the build was created.
// Set via -ldflags "-X github.com/davidallendj/magellan/internal/version.GitBranch=$(git rev-parse --abbrev-ref HEAD)"
var GitBranch string

// GitTag represents the most recent Git tag at build time, if any.
// Set via -ldflags "-X github.com/davidallendj/magellan/internal/version.GitTag=$(git describe --tags --abbrev=0)"
var GitTag string

// GitState indicates whether the working directory was "clean" or "dirty" (i.e., with uncommitted changes).
// Set via -ldflags "-X github.com/davidallendj/magellan/internal/version.GitState=$(if git diff-index --quiet HEAD --; then echo 'clean'; else echo 'dirty'; fi)"
var GitState string

// BuildHost stores the hostname of the machine where the binary was built.
// Set via -ldflags "-X github.com/davidallendj/magellan/internal/version.BuildHost=$(hostname)"
var BuildHost string

// GoVersion captures the Go version used to build the binary.
// Typically, this can be obtained automatically with runtime.Version(), but you can set it manually.
// Set via -ldflags "-X github.com/davidallendj/magellan/internal/version.GoVersion=$(go version | awk '{print $3}')"
var GoVersion string

// BuildUser is the username of the person or system that initiated the build process.
// Set via -ldflags "-X github.com/davidallendj/magellan/internal/version.BuildUser=$(whoami)"
var BuildUser string

// PrintVersionInfo outputs all versioning information for troubleshooting or version checks.
func PrintVersionInfo() {
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Git Commit: %s\n", GitCommit)
	fmt.Printf("Build Time: %s\n", BuildTime)
	fmt.Printf("Git Branch: %s\n", GitBranch)
	fmt.Printf("Git Tag: %s\n", GitTag)
	fmt.Printf("Git State: %s\n", GitState)
	fmt.Printf("Build Host: %s\n", BuildHost)
	fmt.Printf("Go Version: %s\n", GoVersion)
	fmt.Printf("Build User: %s\n", BuildUser)
}

func VersionInfo() string {
	return fmt.Sprintf("Version: %s, Git Commit: %s, Build Time: %s, Git Branch: %s, Git Tag: %s, Git State: %s, Build Host: %s, Go Version: %s, Build User: %s",
		Version, GitCommit, BuildTime, GitBranch, GitTag, GitState, BuildHost, GoVersion, BuildUser)
}
