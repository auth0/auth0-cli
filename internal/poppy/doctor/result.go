// Package doctor holds the types shared by the poppy doctor checks.
package doctor

import (
	"fmt"
	"strings"
)

// Level is the outcome of one check.
type Level string

// The possible outcomes of a check.
const (
	Pass    Level = "pass"
	Warn    Level = "warn"
	Fail    Level = "fail"
	Skipped Level = "skipped"
)

// SkipReason tells why a check did not run. It is set on every skipped result
// and is part of the JSON report, so a script can tell a login that is missing
// from a part of Poppy that is switched off.
type SkipReason string

// The reasons a check is skipped.
const (
	SkipNoLogin         SkipReason = "no_login"
	SkipMissingScope    SkipReason = "missing_scope"
	SkipDifferentTenant SkipReason = "different_tenant"
	SkipWebOff          SkipReason = "web_off"
	SkipAPIOff          SkipReason = "api_off"
	SkipNoConfigFile    SkipReason = "no_config_file"
	SkipNotAvailable    SkipReason = "not_available"
	SkipProtocolVersion SkipReason = "protocol_version"
)

// AgentRef identifies the agent a result is about. The message of the result
// names the agent, and this keeps its URL for a script.
type AgentRef struct {
	ID   string `json:"id,omitempty"`
	URL  string `json:"url,omitempty"`
	Name string `json:"name,omitempty"`
}

// Result is the outcome of one check.
type Result struct {
	ID    string `json:"id"`   // For example "D2". Several results can share it.
	Code  string `json:"code"` // Names the finding and does not change, for example "D8.iss_parameter_off".
	Spec  string `json:"spec"` // Protocol section, for example "3.2".
	Level Level  `json:"level"`
	// SkipReason is set when the level is skipped.
	SkipReason SkipReason `json:"skip_reason,omitempty"`
	Message    string     `json:"message"`
	// FixCommand is a command that can be run as it is, or empty.
	FixCommand string `json:"fix_command,omitempty"`
	// FixHint is a sentence on what to change.
	FixHint            string `json:"fix_hint,omitempty"`
	PlatformDependency bool   `json:"platform_dependency,omitempty"` // True when only Auth0 can fix a failure.
	// Agent is set on a result that is about one agent.
	Agent *AgentRef `json:"agent,omitempty"`
}

// Counts is how many results have each level.
type Counts struct {
	Pass    int `json:"pass"`
	Warn    int `json:"warn"`
	Fail    int `json:"fail"`
	Skipped int `json:"skipped"`
}

// Report is the outcome of all checks for one domain.
type Report struct {
	Domain          string `json:"domain"`
	ProtocolVersion string `json:"protocol_version,omitempty"` // The protocol_version of poppy.json, when it was read.
	Issuer          string `json:"issuer,omitempty"`           // The issuer the checks resolved, when known.
	// TenantChecked is true when the checks that read the tenant (D8 to D14) ran.
	TenantChecked bool `json:"tenant_checked"`
	// TenantOnly is true when only the checks that read the tenant were asked for.
	TenantOnly bool     `json:"tenant_only,omitempty"`
	Totals     Counts   `json:"summary"`
	Results    []Result `json:"results"`
}

// Count returns how many results have the given level.
func (r Report) Count(l Level) int {
	n := 0
	for _, res := range r.Results {
		if res.Level == l {
			n++
		}
	}
	return n
}

// counts tallies the results by level.
func (r Report) counts() Counts {
	return Counts{Pass: r.Count(Pass), Warn: r.Count(Warn), Fail: r.Count(Fail), Skipped: r.Count(Skipped)}
}

// ExitCode returns 1 if any check failed, else 0.
func (r Report) ExitCode() int {
	if r.Count(Fail) > 0 {
		return 1
	}
	return 0
}

// StrictExitCode is ExitCode for a run that also counts the checks that did not
// run as a failure.
func (r Report) StrictExitCode() int {
	if r.Count(Skipped) > 0 {
		return 1
	}
	return r.ExitCode()
}

// Summary returns the counts of the report as one sentence, such as
// "1 failed, 2 warnings, 19 passed, 1 skipped.".
func (r Report) Summary() string {
	if len(r.Results) == 0 {
		return "0 checks ran."
	}

	var parts []string
	if n := r.Count(Fail); n > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", n))
	}
	if n := r.Count(Warn); n > 0 {
		noun := "warning"
		if n > 1 {
			noun += "s"
		}
		parts = append(parts, fmt.Sprintf("%d %s", n, noun))
	}
	parts = append(parts, fmt.Sprintf("%d passed", r.Count(Pass)))
	if n := r.Count(Skipped); n > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", n))
	}

	return strings.Join(parts, ", ") + "."
}
