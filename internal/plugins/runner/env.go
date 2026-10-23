package runner

import (
	"slices"
	"strings"
)

// pluginPrefix starts the name of every variable that the CLI sets for the
// plugin, and that nothing else may set.
const pluginPrefix = "AUTH0_PLUGIN_"

// reservedPrefix starts the name of every variable that belongs to the CLI: its
// credentials, its settings and the variables it sets for the plugin. A command
// can never be given one of them from the user's environment, whatever it
// declared.
const reservedPrefix = "AUTH0_"

// baseNames are the variables of the user's environment that every plugin gets,
// when they are set. Nothing else is passed on unless the command declared it.
var baseNames = []string{
	"PATH", "HOME", "LANG", "LC_ALL", "LC_CTYPE", "TERM", "TMPDIR",
	"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY",
	"http_proxy", "https_proxy", "no_proxy",
}

// windowsNames are added to the base list on Windows, which a plugin needs to
// find its way around the system. They are untested until Windows support lands.
var windowsNames = []string{
	"SystemRoot", "TEMP", "TMP", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "PATHEXT", "ComSpec",
}

// BaseEnv returns the allowlisted variables that lookup has a value for, as
// NAME=value entries. A variable that is unset or empty is left out. The names
// that only exist on Windows are used only when goos is "windows".
func BaseEnv(lookup func(string) string, goos string) []string {
	names := baseNames
	if goos == "windows" {
		names = slices.Concat(baseNames, windowsNames)
	}

	return entries(names, lookup)
}

// DeclaredEnv returns the variables that a command declared and that lookup has a
// value for, as NAME=value entries. A variable that is unset or empty is left
// out, and so is a name that cannot be the name of a variable. A name that starts
// with AUTH0_, in any case, is left out too, so that the credentials of the CLI
// never reach a plugin even from a manifest that asks for them.
func DeclaredEnv(names []string, lookup func(string) string) []string {
	allowed := make([]string, 0, len(names))

	for _, name := range names {
		if !isReserved(name) {
			allowed = append(allowed, name)
		}
	}

	return entries(allowed, lookup)
}

// isReserved reports whether a name starts with AUTH0_, in any case.
func isReserved(name string) bool {
	return strings.HasPrefix(strings.ToUpper(name), reservedPrefix)
}

// entries returns NAME=value for each name that has a value.
func entries(names []string, lookup func(string) string) []string {
	var result []string

	for _, name := range names {
		if name == "" || strings.Contains(name, "=") {
			continue
		}

		if value := lookup(name); value != "" {
			result = append(result, name+"="+value)
		}
	}

	return result
}

// PluginVars turns the values of the CLI-owned flags into the variables the
// plugin reads. The name of a flag becomes AUTH0_PLUGIN_ followed by the name in
// upper case with every dash replaced by an underscore, so json-compact becomes
// AUTH0_PLUGIN_JSON_COMPACT. The result is sorted, so that it is the same on
// every run.
func PluginVars(owned map[string]string) []string {
	result := make([]string, 0, len(owned))

	for name, value := range owned {
		result = append(result, pluginPrefix+strings.ToUpper(strings.ReplaceAll(name, "-", "_"))+"="+value)
	}

	slices.Sort(result)

	return result
}

// EnvSources are the places the environment of a plugin is made from.
type EnvSources struct {
	// Base is the allowlisted part of the user's environment.
	Base []string

	// Declared is the part of the user's environment that the command declared.
	Declared []string

	// Proxy is the environment of the loopback proxy.
	Proxy []string

	// Plugin is the variables made from the CLI-owned flags.
	Plugin []string
}

// Env combines the sources into one environment with each name once, sorted by
// name. The variables of the proxy and of the CLI-owned flags always win: a
// variable from Base or Declared that has the same name is dropped, and so is any
// one from those two that starts with AUTH0_, in any case, because those names
// belong to the CLI and its credentials. Manifest validation already keeps a
// command from declaring such names, and this is the second line of defence.
//
// On Windows names are compared without regard to case, as the system does. An
// entry without a name or without an equals sign is dropped.
func Env(goos string, sources EnvSources) []string {
	key := func(name string) string {
		if goos == "windows" {
			return strings.ToUpper(name)
		}

		return name
	}

	merged := map[string]string{}

	add := func(list []string, trusted bool) {
		for _, entry := range list {
			name, _, found := strings.Cut(entry, "=")
			if !found || name == "" {
				continue
			}

			if !trusted && isReserved(name) {
				continue
			}

			merged[key(name)] = entry
		}
	}

	add(sources.Base, false)
	add(sources.Declared, false)
	add(sources.Proxy, true)
	add(sources.Plugin, true)

	result := make([]string, 0, len(merged))
	for _, entry := range merged {
		result = append(result, entry)
	}

	slices.Sort(result)

	return result
}

// loopbackHosts are the addresses of the proxy of the CLI, which a plugin must
// reach directly.
var loopbackHosts = []string{"127.0.0.1", "localhost", "::1"}

// ExemptLoopback makes sure that NO_PROXY and no_proxy in the environment name the
// loopback addresses, so that a plugin that runs behind a proxy of the user does
// not send its calls to the proxy of the CLI through that proxy. The addresses
// are added to the value the user has, after a comma, when they are not in it
// already, and the variables are set when the user has none. On Windows, where
// names are not case sensitive, there is one variable. The result is sorted.
func ExemptLoopback(env []string, goos string) []string {
	names := []string{"NO_PROXY", "no_proxy"}
	if goos == "windows" {
		names = names[:1]
	}

	result := slices.Clone(env)

	for _, name := range names {
		found := false

		for i, entry := range result {
			current, value, ok := strings.Cut(entry, "=")
			if !ok || !sameName(current, name, goos) {
				continue
			}

			result[i] = current + "=" + withLoopback(value)
			found = true

			break
		}

		if !found {
			result = append(result, name+"="+withLoopback(""))
		}
	}

	slices.Sort(result)

	return result
}

// sameName compares two names of variables as the system does.
func sameName(a, b, goos string) bool {
	if goos == "windows" {
		return strings.EqualFold(a, b)
	}

	return a == b
}

// withLoopback adds the loopback addresses that a comma separated list of hosts
// does not have, after the ones it has.
func withLoopback(list string) string {
	var have []string

	for _, host := range strings.Split(list, ",") {
		if host = strings.TrimSpace(host); host != "" {
			have = append(have, strings.ToLower(host))
		}
	}

	result := strings.TrimRight(strings.TrimSpace(list), ", ")

	for _, host := range loopbackHosts {
		if slices.Contains(have, host) {
			continue
		}

		if result != "" {
			result += ","
		}

		result += host
	}

	return result
}
