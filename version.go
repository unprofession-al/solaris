package main

import "fmt"

var (
	// These variables are passed during `go build` via ldflags, for example:
	//   go build -ldflags "-X main.commit=$(git rev-list -1 HEAD)"
	// goreleaser (https://goreleaser.com/) does this by default.
	version string
	commit  string
	date    string
)

// verisonInfo returns a string containing information usually passed via
// ldflags during build time.
func versionInfo() string {
	if version == "" {
		version = "dirty"
	}
	if commit == "" {
		commit = "dirty"
	}
	if date == "" {
		date = "unknown"
	}
	return fmt.Sprintf("Version:    %s\nCommit:     %s\nBuild Date: %s\n", version, commit, date)
}
