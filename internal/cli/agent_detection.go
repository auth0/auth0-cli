package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// agentEnvEntry maps an env var to a canonical agent_client name.
// requiredPrefix restricts matching to values with that prefix (case-insensitive).
type agentEnvEntry struct {
	envVar         string
	requiredPrefix string
	agentName      string
}

// agentEnvTable is the ordered allow-list of agent env signals. First match wins.
var agentEnvTable = []agentEnvEntry{
	// Claude Code
	{envVar: "CLAUDECODE", agentName: "claude-code"},
	{envVar: "CLAUDE_CODE_ENTRYPOINT", agentName: "claude-code"},
	{envVar: "AI_AGENT", requiredPrefix: "claude-code", agentName: "claude-code"},
	// Cursor
	{envVar: "CURSOR_TRACE_ID", agentName: "cursor"},
	{envVar: "CURSOR_SESSION_ID", agentName: "cursor"},
	{envVar: "TERM_PROGRAM", requiredPrefix: "cursor", agentName: "cursor"},
	// GitHub Copilot (token is a credential, not an agent signal; use COPILOT_AGENT only)
	{envVar: "COPILOT_AGENT", agentName: "github-copilot"},
	// Codex
	{envVar: "OPENAI_CODEX", agentName: "codex"},
	// Gemini (agent-runtime marker only; GEMINI_API_KEY is a credential, not an agent signal)
	{envVar: "GEMINI_CLI_VERSION", agentName: "gemini"},
	// VS Code terminal — TERM_PROGRAM is set by the integrated terminal.
	{envVar: "TERM_PROGRAM", requiredPrefix: "vscode", agentName: "vscode-terminal"},
	// AI_AGENT catch-all (must be last).
	{envVar: "AI_AGENT", agentName: "unknown-agent"},
}

// agentProcessNames maps parent process names (partial, lower-cased) to agent names.
// Covers both AI agent binaries and CLI surfaces that spawn auth0-cli as a subprocess.
var agentProcessNames = map[string]string{
	// AI agents
	"claude":  "claude-code",
	"cursor":  "cursor",
	"copilot": "github-copilot",
	"codex":   "codex",
	"gemini":  "gemini",
	// Auth0 first-party CLI surfaces
	"auth0-mcp-server": "mcp-server", // fallback; handshake is preferred (node removed: too broad)
}

// detectAgent resolves agent_client via a waterfall:
// 1. AUTH0_CLI_CLIENT handshake, 2. env allow-list, 3. parent-process walk, 4. fallback.
func detectAgent(interactive bool) string {
	return detectAgentWithEnv(os.Getenv, os.Getppid, readProcessName, readParentPID, interactive)
}

// detectAgentWithEnv is the testable form, accepting injected env/process readers.
func detectAgentWithEnv(
	getEnv func(string) string,
	getppid func() int,
	procName func(int) string,
	readParentPIDFn func(int) int,
	interactive bool,
) string {
	// Tier 1: Handshake — AUTH0_CLI_CLIENT set by our own surfaces.
	if client := strings.TrimSpace(getEnv("AUTH0_CLI_CLIENT")); client != "" {
		return sanitizeAgentName(client)
	}

	// Tier 2: Env allow-list.
	for _, entry := range agentEnvTable {
		raw := strings.TrimSpace(getEnv(entry.envVar))
		if raw == "" {
			continue
		}

		if entry.requiredPrefix != "" {
			if !strings.HasPrefix(strings.ToLower(raw), strings.ToLower(entry.requiredPrefix)) {
				continue
			}
		}

		return entry.agentName
	}

	// Tier 3: Parent-process walk (up to 3 levels).
	// Note: Tier 2 may return "unknown-agent" (env matched but no specific agent),
	// which is distinct from Tier 4 fallback "unknown" (no signal found).
	pid := getppid()
	for depth := 0; depth < 3 && pid > 1; depth++ {
		name := strings.ToLower(strings.TrimSpace(procName(pid)))
		if name == "" {
			break
		}

		for fragment, agentName := range agentProcessNames {
			if strings.Contains(name, fragment) {
				return agentName
			}
		}

		nextPPID := readParentPIDFn(pid)
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

// readProcessName returns the comm name for a PID. Uses /proc on Linux, ps(1) elsewhere.
func readProcessName(pid int) string {
	switch runtime.GOOS {
	case "linux":
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}

	// macOS and fallback: ps -p <pid> -o comm=
	out, err := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "comm=").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}

	return ""
}

// readParentPID returns the PPID for a given PID.
// Supports Linux (/proc) and macOS (ps). Windows: not implemented, returns 0 (process walk degrades to env-only).
func readParentPID(pid int) int {
	switch runtime.GOOS {
	case "linux":
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
		if err != nil {
			return 0
		}

		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PPid:") {
				var ppid int
				if _, err := fmt.Sscanf(strings.TrimPrefix(line, "PPid:"), "%d", &ppid); err == nil {
					return ppid
				}
			}
		}
	case "darwin":
		// macOS: ppid via ps(1)
		out, err := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "ppid=").Output()
		if err == nil {
			var ppid int
			if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &ppid); err == nil {
				return ppid
			}
		}
	}

	return 0
}

// knownAgentClients is the allow-list for AUTH0_CLI_CLIENT. Extend when adding new surfaces.
var knownAgentClients = []string{
	// Auth0 first-party surfaces
	"mcp-server",
	// AI agents
	"claude-code",
	"cursor",
	"github-copilot",
	"codex",
	"gemini",
	// CLI/terminal surfaces
	"vscode-terminal",
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
