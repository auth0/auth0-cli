package buildinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDefaultBuildInfo(t *testing.T) {
	assert.Contains(t, NewDefaultBuildInfo().GoVersion, "go1.")
}

func TestNewBuildInfo(t *testing.T) {
	mockBuildInfo := NewBuildInfo("mock-version", "mock-branch", "mock-build-date", "mock-build-user", "mock-go-version", "mock-revision")

	assert.Equal(t, mockBuildInfo, BuildInfo{
		Version:   "mock-version",
		Branch:    "mock-branch",
		BuildDate: "mock-build-date",
		BuildUser: "mock-build-user",
		GoVersion: "mock-go-version",
		Revision:  "mock-revision",
	})
}
