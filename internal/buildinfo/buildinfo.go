// Package buildinfo carries the values the linker stamps into a release build.
package buildinfo

import "runtime/debug"

// Overridden at build time with -ldflags "-X ...buildinfo.Version=1.2.3".
var (
	Version = "0.1.0"
	Commit  = ""
	Date    = ""
)

// Revision is the commit the binary was built from. A `go install` build has
// no ldflags, so it falls back to the VCS stamp the toolchain embeds.
func Revision() string {
	if Commit != "" {
		return Commit
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			return setting.Value
		}
	}
	return "unknown"
}

// BuildDate is when the binary was built, from ldflags or the VCS stamp.
func BuildDate() string {
	if Date != "" {
		return Date
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.time" {
			return setting.Value
		}
	}
	return "unknown"
}

// UserAgent identifies the CLI to the API, ahead of the SDK's own token.
func UserAgent() string { return "fopost-cli/" + Version }
