package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// agentClientUnknownAgent is returned when an agent was detected but could not be named.
// It is distinct from "unknown", which means no invoker signal was found at all.
const agentClientUnknownAgent = "unknown-agent"

// envCondition is one requirement on an env var: presence by default, an exact value
// via requiredValue, or a case-insensitive prefix via requiredPrefix.
type envCondition struct {
	envVar         string
	requiredValue  string
	requiredPrefix string
}

// agentEnvEntry maps a set of AND-ed env conditions to a canonical agent_client name.
type agentEnvEntry struct {
	conditions []envCondition
	agentName  string
}

// matches reports whether every condition holds. An entry with no conditions never matches.
func (e agentEnvEntry) matches(getEnv func(string) string) bool {
	for _, cond := range e.conditions {
		raw := strings.TrimSpace(getEnv(cond.envVar))
		switch {
		case raw == "":
			return false
		case cond.requiredValue != "" && raw != cond.requiredValue:
			return false
		case cond.requiredPrefix != "" && !strings.HasPrefix(strings.ToLower(raw), strings.ToLower(cond.requiredPrefix)):
			return false
		}
	}

	return len(e.conditions) > 0
}

// agentEnvTable is the ordered allow-list of agent env signals. First match wins.
var agentEnvTable = []agentEnvEntry{
	// Claude Code.
	{conditions: []envCondition{{envVar: "CLAUDECODE"}}, agentName: "claude-code"},
	{conditions: []envCondition{{envVar: "CLAUDE_CODE_SESSION_ID"}}, agentName: "claude-code"},
	{conditions: []envCondition{{envVar: "CLAUDE_CODE_ENTRYPOINT"}}, agentName: "claude-code"},
	{conditions: []envCondition{{envVar: "AI_AGENT", requiredPrefix: "claude-code"}}, agentName: "claude-code"},
	// Cursor — the agent, not a human in its terminal. CURSOR_TRACE_ID is set for every
	// Cursor terminal, so we key on the agent-execution markers instead: CURSOR_AGENT
	// (headless cursor-agent CLI) and CURSOR_EXTENSION_HOST_ROLE=agent-exec (in-IDE agent).
	{conditions: []envCondition{{envVar: "CURSOR_AGENT"}}, agentName: "cursor"},
	{conditions: []envCondition{{envVar: "CURSOR_EXTENSION_HOST_ROLE", requiredValue: "agent-exec"}}, agentName: "cursor"},
	// Codex.
	{conditions: []envCondition{{envVar: "CODEX_THREAD_ID"}}, agentName: "codex"},
	// Gemini-cli.
	{conditions: []envCondition{{envVar: "GEMINI_CLI_VERSION"}}, agentName: "gemini"},
	// AntiGravity.
	{conditions: []envCondition{{envVar: "ANTIGRAVITY_CLI_ALIAS"}}, agentName: "antigravity"},
	{conditions: []envCondition{{envVar: "ANTIGRAVITY_CONVERSATION_ID"}}, agentName: "antigravity"},
	// AI_AGENT catch-all (must be last).
	{conditions: []envCondition{{envVar: "AI_AGENT"}}, agentName: agentClientUnknownAgent},
}

// agentProcessNames maps parent process names (partial, lower-cased) to agent names.
// Cursor is omitted: its app is the ancestor of both a human terminal and the agent,
// so the process name cannot tell them apart; the env markers above handle Cursor.
var agentProcessNames = map[string]string{
	// AI agents.
	"claude":  "claude-code",
	"copilot": "github-copilot",
	"codex":   "codex",
	"gemini":  "gemini",
	"agy":     "antigravity",
	// Auth0 first-party CLI surfaces.
	"auth0-mcp-server": "mcp-server",
}

// detectAgent resolves agent_client via a waterfall:
// Tier 1 AUTH0_CLI_CLIENT handshake, Tier 2 env allow-list, Tier 3 parent-process walk, Tier 4 fallback.
func detectAgent(interactive bool) string {
	return detectAgentWithEnv(os.Getenv, os.Environ, os.Getppid, getProcInfo, interactive)
}

// agentEnvSuffixes are naming conventions shared across agent CLIs. Matching any of
// these on an env var key signals an agent we don't yet have a named entry for.
var agentEnvSuffixes = []string{
	"_CONVERSATION_ID",
	"_THREAD_ID",
	"_AGENT_SESSION_ID",
}

// detectAgentWithEnv is the testable form, accepting injected env/process readers.
func detectAgentWithEnv(
	getEnv func(string) string,
	environ func() []string,
	getppid func() int,
	procInfo func(int) (string, int),
	interactive bool,
) string {
	// Tier 1: Handshake — AUTH0_CLI_CLIENT set by our own surfaces.
	if client := strings.TrimSpace(getEnv("AUTH0_CLI_CLIENT")); client != "" {
		return sanitizeAgentName(client)
	}

	// Tier 2: Env allow-list.
	for _, entry := range agentEnvTable {
		if entry.matches(getEnv) {
			return entry.agentName
		}
	}

	// Tier 2b: Wildcard sweep for unknown future agents. Catches the shared
	// naming conventions (*_CONVERSATION_ID / *_THREAD_ID / *_AGENT_SESSION_ID)
	// without a per-agent code change. Returns the generic "unknown-agent".
	for _, kv := range environ() {
		key, val, ok := strings.Cut(kv, "=")
		if !ok || strings.TrimSpace(val) == "" {
			continue
		}

		upperKey := strings.ToUpper(key)
		// Skip CURSOR_* vars: they're set for every Cursor terminal, human included, so
		// they must not trip the sweep. The Cursor agent is matched in Tier 2 above.
		if strings.HasPrefix(upperKey, "CURSOR_") {
			continue
		}
		for _, suffix := range agentEnvSuffixes {
			if strings.HasSuffix(upperKey, suffix) {
				return agentClientUnknownAgent
			}
		}
	}

	// Tier 3: Parent-process walk (up to 3 levels).
	// Note: Tier 2 may return "unknown-agent" (env matched but no specific agent),
	// which is distinct from Tier 4 fallback "unknown" (no signal found).
	pid := getppid()
	for depth := 0; depth < 3 && pid > 1; depth++ {
		rawName, nextPPID := procInfo(pid)
		name := strings.ToLower(strings.TrimSpace(rawName))
		if name == "" {
			break
		}

		for fragment, agentName := range agentProcessNames {
			if strings.Contains(name, fragment) {
				return agentName
			}
		}

		if nextPPID <= 1 {
			break
		}

		pid = nextPPID
	}

	// Tier 4: Fallback.
	if !interactive {
		return "unknown"
	}

	return "human"
}

type procInfo struct {
	ppid int
	name string
}

var (
	procCache   = make(map[int]procInfo)
	procCacheMu sync.Mutex
)

// getProcInfo returns the process name and parent PID for a PID, cached to avoid
// repeat lookups. Linux and macOS only; elsewhere returns ("", 0).
func getProcInfo(pid int) (string, int) {
	procCacheMu.Lock()
	defer procCacheMu.Unlock()

	if info, ok := procCache[pid]; ok {
		return info.name, info.ppid
	}

	var name string
	var ppid int

	switch runtime.GOOS {
	case "linux":
		commData, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err == nil {
			name = strings.TrimSpace(string(commData))
		}

		statusData, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
		if err == nil {
			for _, line := range strings.Split(string(statusData), "\n") {
				if strings.HasPrefix(line, "PPid:") {
					var parsedPPID int
					if _, err := fmt.Sscanf(strings.TrimPrefix(line, "PPid:"), "%d", &parsedPPID); err == nil {
						ppid = parsedPPID
						break
					}
				}
			}
		}
	case "darwin":
		// On macOS, query PPID and command name in a single ps invocation to avoid an extra process spawn.
		out, err := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "ppid=", "-o", "comm=").Output()
		if err == nil {
			fields := strings.Fields(strings.TrimSpace(string(out)))
			if len(fields) >= 2 {
				var parsedPPID int
				if _, err := fmt.Sscanf(fields[0], "%d", &parsedPPID); err == nil {
					ppid = parsedPPID
				}
				name = strings.Join(fields[1:], " ")
			}
		}
	}

	procCache[pid] = procInfo{name: name, ppid: ppid}
	return name, ppid
}

// knownAgentClients is the allow-list for AUTH0_CLI_CLIENT. Extend when adding new surfaces.
var knownAgentClients = []string{
	// Auth0 first-party surfaces.
	"mcp-server",
	"claude-code",
	"cursor",
	"github-copilot",
	"codex",
	"gemini",
	"antigravity",
}

// sanitizeAgentName restricts AUTH0_CLI_CLIENT to the allow-list; unknown values are prefixed with "client-".
func sanitizeAgentName(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))

	for _, name := range knownAgentClients {
		if lower == name {
			return name
		}
	}

	return "client-" + lower
}
