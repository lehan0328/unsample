// Package version holds the build version, injected at compile time via ldflags.
package version

// Version is the current build version, set by the linker.
// Example: go build -ldflags '-X github.com/unsample/unsample/internal/version.Version=v0.1.0'
var Version = "dev"

// Commit is the git short commit hash, set by the linker.
var Commit = "none"
