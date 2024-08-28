package main

import (
	"github.com/OpenCHAMI/magellan/cmd"
)

var (
	version string
	commit  string
	date    string
)

func main() {
	cmd.SetVersionInfo(version, commit, date)
	cmd.Execute()
}
