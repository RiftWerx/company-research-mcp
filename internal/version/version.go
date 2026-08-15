// Package version holds the binary version, injectable via ldflags at build time.
package version

import "runtime/debug"

// Version defaults to "dev" for local builds; goreleaser and make local-release
// override it via -ldflags "-X github.com/riftwerx/company-research/internal/version.Version=<tag>".
var Version = "dev"

// String returns the effective version: the ldflags-injected Version if one was
// set, otherwise the module version Go embeds automatically for "go install
// module@version" builds. Falls back to "dev" for builds that have neither
// (e.g. "go build" run from a local checkout).
func String() string {
	if Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return Version
}
