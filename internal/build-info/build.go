package buildinfo

import (
	"runtime"
)

var (
	Version   string
	Revision  string
	Branch    string
	BuildUser string
	BuildDate string
	GoVersion = runtime.Version()
)

type BuildInfo struct {
	Version   string
	Revision  string
	Branch    string
	BuildUser string
	BuildDate string
	GoVersion string
}

// NewDefaultBuildInfo returns the build information obtained from ldflags
func NewDefaultBuildInfo() BuildInfo {
	return NewBuildInfo(Version, Branch, BuildDate, BuildUser, GoVersion, Revision)
}

// NewBuildInfo returns an object with the build information
func NewBuildInfo(version, branch, buildDate, buildUser, goVersion, revision string) BuildInfo {
	return BuildInfo{
		Version:   version,
		Branch:    branch,
		BuildDate: buildDate,
		BuildUser: buildUser,
		GoVersion: goVersion,
		Revision:  revision,
	}
}
