package plugins

import (
	"debug/elf"
	"errors"
	"runtime"
	"strings"
)

// LibcDetector reports the C library of the running system, which is "glibc" or
// "musl". It is only called on Linux.
type LibcDetector func() (string, error)

// CurrentPlatform returns the platform that the CLI runs on. It reports false
// when no platform of a plugin matches this machine, including a Linux system
// whose C library cannot be told.
func CurrentPlatform() (Platform, bool) {
	return DetectPlatform(runtime.GOOS, runtime.GOARCH, DetectLibc)
}

// DetectPlatform is CurrentPlatform for the given Go operating system and
// architecture names, using detect to find the C library on Linux. It reports
// false for a combination that is not a platform, and when detect fails or
// reports a library that is not supported.
func DetectPlatform(goos, goarch string, detect LibcDetector) (Platform, bool) {
	var arch string

	switch goarch {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	default:
		return "", false
	}

	libc := ""

	if goos == "linux" {
		var err error

		if libc, err = detect(); err != nil {
			return "", false
		}
	}

	return PlatformFor(goos, arch, libc)
}

// errLibcUnknown means the C library could not be told from the system.
var errLibcUnknown = errors.New("could not tell which C library this system uses")

// interpreterSources are the executables whose ELF interpreter names the C
// library of the system. The shell is dynamically linked on every mainstream
// Linux, including Alpine, where it is a BusyBox built against musl. The CLI
// itself is tried last because a static build has no interpreter.
var interpreterSources = []string{"/bin/sh", "/proc/self/exe"}

// DetectLibc tells glibc from musl by the dynamic loader that the system's own
// executables ask for.
func DetectLibc() (string, error) {
	for _, source := range interpreterSources {
		if libc, ok := libcOfExecutable(source); ok {
			return libc, nil
		}
	}

	return "", errLibcUnknown
}

// libcOfExecutable reads the ELF interpreter of the executable at path and
// reports the C library it belongs to.
func libcOfExecutable(path string) (string, bool) {
	file, err := elf.Open(path)
	if err != nil {
		return "", false
	}
	defer file.Close()

	section := file.Section(".interp")
	if section == nil || section.Size == 0 || section.Size > 4096 {
		return "", false
	}

	data, err := section.Data()
	if err != nil {
		return "", false
	}

	return libcOfInterpreter(strings.TrimRight(string(data), "\x00"))
}

// libcOfInterpreter maps the path of a dynamic loader to its C library, for
// example /lib/ld-musl-x86_64.so.1 to musl and /lib64/ld-linux-x86-64.so.2 to
// glibc.
func libcOfInterpreter(interpreter string) (string, bool) {
	name := interpreter[strings.LastIndex(interpreter, "/")+1:]

	switch {
	case strings.HasPrefix(name, "ld-musl-"):
		return libcMusl, true
	case strings.HasPrefix(name, "ld-linux"):
		return libcGlibc, true
	default:
		return "", false
	}
}
