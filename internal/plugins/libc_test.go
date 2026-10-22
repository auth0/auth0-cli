package plugins

import (
	"errors"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectPlatform(t *testing.T) {
	glibc := func() (string, error) { return "glibc", nil }
	musl := func() (string, error) { return "musl", nil }
	failing := func() (string, error) { return "", errors.New("no loader") }
	unexpected := func() (string, error) { t.Fatal("the detector must only run on Linux"); return "", nil }

	tests := []struct {
		name         string
		goos, goarch string
		detect       LibcDetector
		want         Platform
		ok           bool
	}{
		{"linux x64 glibc", "linux", "amd64", glibc, PlatformLinuxX64, true},
		{"linux x64 musl", "linux", "amd64", musl, PlatformLinuxX64Musl, true},
		{"linux arm64 glibc", "linux", "arm64", glibc, PlatformLinuxArm64, true},
		{"linux arm64 musl", "linux", "arm64", musl, PlatformLinuxArm64Musl, true},
		{"linux with an undetectable library", "linux", "amd64", failing, "", false},
		{"linux with an unsupported library", "linux", "amd64", func() (string, error) { return "uclibc", nil }, "", false},
		{"darwin arm64 does not ask the detector", "darwin", "arm64", unexpected, PlatformDarwinArm64, true},
		{"darwin x64", "darwin", "amd64", unexpected, PlatformDarwinX64, true},
		{"windows x64", "windows", "amd64", unexpected, PlatformWindowsX64, true},
		{"windows arm64 is not a platform", "windows", "arm64", unexpected, "", false},
		{"unknown architecture", "linux", "riscv64", glibc, "", false},
		{"unknown system", "freebsd", "amd64", unexpected, "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := DetectPlatform(test.goos, test.goarch, test.detect)

			assert.Equal(t, test.ok, ok)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestLibcOfInterpreter(t *testing.T) {
	tests := []struct {
		interpreter string
		want        string
		ok          bool
	}{
		{"/lib64/ld-linux-x86-64.so.2", "glibc", true},
		{"/lib/ld-linux-aarch64.so.1", "glibc", true},
		{"/nix/store/abc-glibc-2.39/lib/ld-linux-x86-64.so.2", "glibc", true},
		{"/lib/ld-musl-x86_64.so.1", "musl", true},
		{"/lib/ld-musl-aarch64.so.1", "musl", true},
		{"/usr/lib/something-else.so", "", false},
		{"", "", false},
	}

	for _, test := range tests {
		t.Run(test.interpreter, func(t *testing.T) {
			got, ok := libcOfInterpreter(test.interpreter)

			assert.Equal(t, test.ok, ok)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestDetectLibc_OnThisSystem(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only Linux has a C library to detect")
	}

	libc, err := DetectLibc()

	require.NoError(t, err)
	assert.Contains(t, []string{"glibc", "musl"}, libc)
}

func TestLibcOfExecutable_RejectsFilesThatAreNotElf(t *testing.T) {
	_, ok := libcOfExecutable("/does/not/exist")
	assert.False(t, ok)

	_, ok = libcOfExecutable(t.TempDir())
	assert.False(t, ok)
}

func TestCurrentPlatform_IsAKnownPlatformOnSupportedMachines(t *testing.T) {
	platform, ok := CurrentPlatform()
	if !ok {
		t.Skip("this machine is not one of the plugin platforms")
	}

	assert.Contains(t, AllPlatforms(), platform)
	assert.Equal(t, runtime.GOOS, platform.OS())
}
