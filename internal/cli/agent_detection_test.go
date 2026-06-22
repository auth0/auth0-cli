package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func noEnv(string) string { return "" }
func noEnviron() []string { return nil }
func noProc(int) string   { return "" }
func dummyPPID() int      { return 9999 }

// noProcInfo is a process reader that returns no name and no parent PID.
func noProcInfo(int) (string, int) { return "", 0 }

// procInfoName adapts a name-only lookup into the combined (name, ppid) reader,
// reporting no parent so the walk stops after one level.
func procInfoName(procName func(int) string) func(int) (string, int) {
	return func(pid int) (string, int) { return procName(pid), 0 }
}

func detectAgentFull(getEnv func(string) string, procName func(int) string, interactive bool) string {
	return detectAgentWithEnv(getEnv, noEnviron, dummyPPID, procInfoName(procName), interactive)
}

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

func TestDetectAgent_ClaudeCode_CLAUDECODE(t *testing.T) {
	agent := detectAgentFull(func(k string) string {
		if k == "CLAUDECODE" {
			return "1"
		}
		return ""
	}, noProc, false)
	assert.Equal(t, "claude-code", agent)
}

func TestDetectAgent_ClaudeCode_SessionID(t *testing.T) {
	agent := detectAgentFull(func(k string) string {
		if k == "CLAUDE_CODE_SESSION_ID" {
			return "session_123"
		}
		return ""
	}, noProc, false)
	assert.Equal(t, "claude-code", agent)
}

func TestDetectAgent_ClaudeCode_Entrypoint(t *testing.T) {
	agent := detectAgentFull(func(k string) string {
		if k == "CLAUDE_CODE_ENTRYPOINT" {
			return "cli"
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
	for _, tc := range []struct {
		envVar string
		value  string
	}{
		{"CURSOR_AGENT", "1"},
		{"CURSOR_TRACE_ID", "abc123"},
		{"CURSOR_CONVERSATION_ID", "04bb112f-88b6-47ce-b23c-2fb28b9b98e3"},
	} {
		t.Run(tc.envVar, func(t *testing.T) {
			agent := detectAgentFull(func(k string) string {
				if k == tc.envVar {
					return tc.value
				}
				return ""
			}, noProc, false)
			assert.Equal(t, "cursor", agent)
		})
	}
}

func TestDetectAgent_CursorTraceIDBeatsWildcard(t *testing.T) {
	agent := detectAgentWithEnv(func(k string) string {
		if k == "CURSOR_TRACE_ID" {
			return "abc123"
		}
		return ""
	}, func() []string {
		return []string{"CURSOR_TRACE_ID=abc123"}
	}, dummyPPID, noProcInfo, false)
	assert.Equal(t, "cursor", agent)
}

func TestDetectAgent_CursorConversationIDBeatsWildcard(t *testing.T) {
	agent := detectAgentWithEnv(func(k string) string {
		if k == "CURSOR_CONVERSATION_ID" {
			return "04bb112f-88b6-47ce-b23c-2fb28b9b98e3"
		}
		return ""
	}, func() []string {
		return []string{"CURSOR_CONVERSATION_ID=04bb112f-88b6-47ce-b23c-2fb28b9b98e3"}
	}, dummyPPID, noProcInfo, false)
	assert.Equal(t, "cursor", agent)
}

func TestDetectAgent_UnlistedCursorInfraIgnoredByWildcard(t *testing.T) {
	agent := detectAgentWithEnv(noEnv, func() []string {
		return []string{"CURSOR_SANDBOX=seatbelt"}
	}, dummyPPID, noProcInfo, false)
	assert.Equal(t, "unknown", agent)
}

func TestDetectAgent_Codex_ThreadID(t *testing.T) {
	agent := detectAgentFull(func(k string) string {
		if k == "CODEX_THREAD_ID" {
			return "thr_123"
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

func TestDetectAgent_Antigravity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		envVar string
		value  string
	}{
		{name: "Alias", envVar: "ANTIGRAVITY_CLI_ALIAS", value: "agy"},
		{name: "ConversationID", envVar: "ANTIGRAVITY_CONVERSATION_ID", value: "conv_123"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent := detectAgentFull(func(k string) string {
				if k == tc.envVar {
					return tc.value
				}
				return ""
			}, noProc, false)
			assert.Equal(t, "antigravity", agent)
		})
	}
}

func TestDetectAgent_ProcessWalk_Claude(t *testing.T) {
	agent := detectAgentWithEnv(noEnv, noEnviron, dummyPPID, procInfoName(func(pid int) string {
		if pid == 9999 {
			return "claude"
		}
		return ""
	}), false)
	assert.Equal(t, "claude-code", agent)
}

func TestDetectAgent_ProcessWalk_MultiLevel(t *testing.T) {
	// Multi-level walk: immediate parent (9999) is zsh (no match),
	// grandparent (9998) is claude (should match at depth 1).
	getppid := func() int { return 9999 }
	procInfo := func(pid int) (string, int) {
		switch pid {
		case 9999:
			return "zsh", 9998 // No name match; parent is 9998.
		case 9998:
			return "claude", 0 // Should match at depth 1.
		}
		return "", 0
	}
	agent := detectAgentWithEnv(noEnv, noEnviron, getppid, procInfo, false)
	assert.Equal(t, "claude-code", agent)
}

func TestDetectAgent_ProcessWalk_Cursor(t *testing.T) {
	agent := detectAgentWithEnv(noEnv, noEnviron, dummyPPID, procInfoName(func(pid int) string {
		if pid == 9999 {
			return "cursor"
		}
		return ""
	}), false)
	assert.Equal(t, "cursor", agent)
}

func TestDetectAgent_ProcessWalk_GitHubCopilot(t *testing.T) {
	agent := detectAgentWithEnv(noEnv, noEnviron, dummyPPID, procInfoName(func(pid int) string {
		if pid == 9999 {
			return "copilot"
		}
		return ""
	}), false)
	assert.Equal(t, "github-copilot", agent)
}

func TestDetectAgent_ProcessWalk_MCPServer(t *testing.T) {
	agent := detectAgentWithEnv(noEnv, noEnviron, dummyPPID, procInfoName(func(pid int) string {
		if pid == 9999 {
			return "auth0-mcp-server"
		}
		return ""
	}), false)
	assert.Equal(t, "mcp-server", agent)
}

func TestDetectAgent_WildcardSweep_Suffixes(t *testing.T) {
	for _, key := range []string{
		"SOMETOOL_CONVERSATION_ID",
		"sometool_thread_id",
		"FUTURE_AGENT_SESSION_ID",
	} {
		agent := detectAgentWithEnv(noEnv, func() []string {
			return []string{key + "=value"}
		}, dummyPPID, noProcInfo, false)
		assert.Equal(t, "unknown-agent", agent, "key: %s", key)
	}
}

func TestDetectAgent_WildcardSweep_EmptyValueIgnored(t *testing.T) {
	// A matching key with an empty value must not trigger a match.
	agent := detectAgentWithEnv(noEnv, func() []string {
		return []string{"SOMETOOL_THREAD_ID="}
	}, dummyPPID, noProcInfo, false)
	assert.Equal(t, "unknown", agent)
}

func TestDetectAgent_WildcardSweep_NoFalsePositive(t *testing.T) {
	// An unrelated env var must not trip the suffix sweep.
	agent := detectAgentWithEnv(noEnv, func() []string {
		return []string{"PATH=/usr/bin", "HOME=/root"}
	}, dummyPPID, noProcInfo, true)
	assert.Equal(t, "human", agent)
}

func TestDetectAgent_NamedEntryBeatsWildcard(t *testing.T) {
	// A named env entry must win over the generic wildcard sweep.
	agent := detectAgentWithEnv(func(k string) string {
		if k == "CODEX_THREAD_ID" {
			return "thr_1"
		}
		return ""
	}, func() []string {
		return []string{"CODEX_THREAD_ID=thr_1"}
	}, dummyPPID, noProcInfo, false)
	assert.Equal(t, "codex", agent)
}

func TestDetectAgent_Fallback_NonInteractive(t *testing.T) {
	agent := detectAgentFull(noEnv, noProc, false)
	assert.Equal(t, "unknown", agent)
}

func TestDetectAgent_Fallback_Interactive(t *testing.T) {
	agent := detectAgentFull(noEnv, noProc, true)
	assert.Equal(t, "human", agent)
}

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
