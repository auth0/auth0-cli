package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func noEnv(string) string { return "" }
func noProc(int) string   { return "" }
func dummyPPID() int      { return 9999 }
func noParentPID(int) int { return 0 }

func detectAgentFull(getEnv func(string) string, procName func(int) string, interactive bool) string {
	return detectAgentWithEnv(getEnv, dummyPPID, procName, noParentPID, interactive)
}

// ─── Tier 1: Handshake ─────────────────────────────────────────────────────

func TestDetectAgent_HandshakeMCPServer(t *testing.T) {
	agent := detectAgentFull(func(k string) string {
		if k == "AUTH0_CLI_CLIENT" {
			return "mcp-server"
		}
		return ""
	}, noProc, false)
	assert.Equal(t, "mcp-server", agent)
}

func TestDetectAgent_HandshakePrecedence(t *testing.T) {
	// Handshake must beat any env-table signal.
	agent := detectAgentFull(func(k string) string {
		switch k {
		case "AUTH0_CLI_CLIENT":
			return "cursor"
		case "CLAUDECODE":
			return "1"
		}
		return ""
	}, noProc, false)
	assert.Equal(t, "cursor", agent)
}

func TestDetectAgent_HandshakeUnknownClientPrefixed(t *testing.T) {
	agent := detectAgentFull(func(k string) string {
		if k == "AUTH0_CLI_CLIENT" {
			return "my-internal-tool"
		}
		return ""
	}, noProc, false)
	assert.Equal(t, "client-my-internal-tool", agent)
}

// ─── Tier 2: Env allow-list ────────────────────────────────────────────────

func TestDetectAgent_ClaudeCode_CLAUDECODE(t *testing.T) {
	agent := detectAgentFull(func(k string) string {
		if k == "CLAUDECODE" {
			return "1"
		}
		return ""
	}, noProc, false)
	assert.Equal(t, "claude-code", agent)
}

func TestDetectAgent_ClaudeCode_AIAgent(t *testing.T) {
	agent := detectAgentFull(func(k string) string {
		if k == "AI_AGENT" {
			return "claude-code_1.2.0"
		}
		return ""
	}, noProc, false)
	assert.Equal(t, "claude-code", agent)
}

func TestDetectAgent_Cursor(t *testing.T) {
	agent := detectAgentFull(func(k string) string {
		if k == "CURSOR_TRACE_ID" {
			return "abc123"
		}
		return ""
	}, noProc, false)
	assert.Equal(t, "cursor", agent)
}

func TestDetectAgent_GitHubCopilot_COPILOT_AGENT(t *testing.T) {
	agent := detectAgentFull(func(k string) string {
		if k == "COPILOT_AGENT" {
			return "1"
		}
		return ""
	}, noProc, false)
	assert.Equal(t, "github-copilot", agent)
}

func TestDetectAgent_Codex(t *testing.T) {
	agent := detectAgentFull(func(k string) string {
		if k == "OPENAI_CODEX" {
			return "1"
		}
		return ""
	}, noProc, false)
	assert.Equal(t, "codex", agent)
}

func TestDetectAgent_Gemini(t *testing.T) {
	agent := detectAgentFull(func(k string) string {
		if k == "GEMINI_CLI_VERSION" {
			return "0.1.0"
		}
		return ""
	}, noProc, false)
	assert.Equal(t, "gemini", agent)
}

func TestDetectAgent_Cursor_TERM_PROGRAM(t *testing.T) {
	agent := detectAgentFull(func(k string) string {
		if k == "TERM_PROGRAM" {
			return "cursor"
		}
		return ""
	}, noProc, false)
	assert.Equal(t, "cursor", agent)
}

func TestDetectAgent_VSCodeTerminal(t *testing.T) {
	agent := detectAgentFull(func(k string) string {
		if k == "TERM_PROGRAM" {
			return "vscode"
		}
		return ""
	}, noProc, false)
	assert.Equal(t, "vscode-terminal", agent)
}

func TestDetectAgent_ProcessWalk_Claude(t *testing.T) {
	agent := detectAgentWithEnv(noEnv, dummyPPID, func(pid int) string {
		if pid == 9999 {
			return "claude"
		}
		return ""
	}, noParentPID, false)
	assert.Equal(t, "claude-code", agent)
}

func TestDetectAgent_ProcessWalk_MultiLevel(t *testing.T) {
	// Multi-level walk: immediate parent (9999) is zsh (no match),
	// grandparent (9998) is claude (should match at depth 1).
	getppid := func() int { return 9999 }
	procName := func(pid int) string {
		switch pid {
		case 9999:
			return "zsh" // No match in agentProcessNames
		case 9998:
			return "claude" // Should match at depth 1
		}
		return ""
	}
	readParentPID := func(pid int) int {
		if pid == 9999 {
			return 9998
		}
		return 0
	}
	agent := detectAgentWithEnv(noEnv, getppid, procName, readParentPID, false)
	assert.Equal(t, "claude-code", agent)
}
func TestDetectAgent_ProcessWalk_Cursor(t *testing.T) {
	agent := detectAgentWithEnv(noEnv, dummyPPID, func(pid int) string {
		if pid == 9999 {
			return "cursor"
		}
		return ""
	}, noParentPID, false)
	assert.Equal(t, "cursor", agent)
}

func TestDetectAgent_ProcessWalk_MCPServer(t *testing.T) {
	agent := detectAgentWithEnv(noEnv, dummyPPID, func(pid int) string {
		if pid == 9999 {
			return "auth0-mcp-server"
		}
		return ""
	}, noParentPID, false)
	assert.Equal(t, "mcp-server", agent)
}

// ─── Tier 4: Fallback ─────────────────────────────────────────────────────

func TestDetectAgent_Fallback_NonInteractive(t *testing.T) {
	agent := detectAgentFull(noEnv, noProc, false)
	assert.Equal(t, "unknown", agent)
}

func TestDetectAgent_Fallback_Interactive(t *testing.T) {
	agent := detectAgentFull(noEnv, noProc, true)
	assert.Equal(t, "human", agent)
}

// ─── Sanitize ─────────────────────────────────────────────────────────────

func TestSanitizeAgentName_KnownNames(t *testing.T) {
	for input, want := range map[string]string{
		"mcp-server":     "mcp-server",
		"MCP-SERVER":     "mcp-server",
		"claude-code":    "claude-code",
		"cursor":         "cursor",
		"github-copilot": "github-copilot",
		"codex":          "codex",
		"gemini":         "gemini",
	} {
		assert.Equal(t, want, sanitizeAgentName(input), "input: %s", input)
	}
}

func TestSanitizeAgentName_UnknownPrefixed(t *testing.T) {
	assert.Equal(t, "client-my-tool", sanitizeAgentName("my-tool"))
}
