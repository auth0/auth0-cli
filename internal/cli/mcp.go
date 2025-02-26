package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

// Structs for Claude Desktop configuration
type claudeDesktopConfig struct {
	MCPServers map[string]claudeMCPServer `json:"mcpServers"`
}

type claudeMCPServer struct {
	Args         []string          `json:"args"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Command      string            `json:"command"`
	Env          map[string]string `json:"env,omitempty"`
}

// Structure for the auth0-mcp-server package.json
type mcpPackageJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCP returns commands for MCP-related operations
func (c *cli) MCP() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Tools for Claude Desktop MCP server integration",
		Long:  "Tools for setting up and managing the Auth0 MCP server integration with Claude Desktop",
	}

	cmd.AddCommand(mcpInitCmd(c))
	return cmd
}

func mcpInitCmd(cli *cli) *cobra.Command {
	var nodePath string
	var serverPath string
	var configPath string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Configure Claude Desktop to use Auth0 MCP server",
		Long:  "Initialize Auth0 MCP server integration with Claude Desktop",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize config
			if err := cli.Config.Initialize(); err != nil {
				return fmt.Errorf("failed to load Auth0 CLI configuration: %w", err)
			}

			// Check if user is logged in
			if !cli.Config.IsLoggedInWithTenant(cli.tenant) {
				cli.renderer.Infof("You need to log in first. Running auth0 login...")
				loginCmd := loginCmd(cli)
				if err := loginCmd.Execute(); err != nil {
					return fmt.Errorf("login failed: %w", err)
				}
				// Reload config after login
				if err := cli.Config.Initialize(); err != nil {
					return fmt.Errorf("failed to reload configuration after login: %w", err)
				}
			}

			// Get tenant
			tenantName := cli.tenant
			if tenantName == "" {
				tenantName = cli.Config.DefaultTenant
			}
			tenant, err := cli.Config.GetTenant(tenantName)
			if err != nil {
				return fmt.Errorf("failed to get tenant %q: %w", tenantName, err)
			}
			
			// Debug token
			token := tenant.GetAccessToken()
			if cli.debug {
				tokenLength := len(token)
				maskedToken := "no token"
				if tokenLength > 0 {
					if tokenLength > 10 {
						maskedToken = token[:5] + "..." + token[tokenLength-5:]
					} else {
						maskedToken = token
					}
				}
				cli.renderer.Infof("Debug - Token info: length=%d, preview=%s", tokenLength, maskedToken)
			}

			// Find server path if not provided
			if serverPath == "" {
				// Try to find in common locations
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get home directory: %w", err)
				}
				
				// Check common installation paths
				commonPaths := []string{
					filepath.Join(homeDir, "dev", "mcp", "auth0-mcp-server", "dist", "index.js"),
					filepath.Join(homeDir, "go", "src", "github.com", "auth0", "auth0-cli", "mcp-server", "dist", "index.js"),
					filepath.Join(homeDir, "Auth0", "auth0-mcp-server", "dist", "index.js"),
				}
				
				for _, path := range commonPaths {
					if _, err := os.Stat(path); err == nil {
						serverPath = path
						cli.renderer.Infof("Found Auth0 MCP server at: %s", serverPath)
						break
					}
				}
				
				if serverPath == "" {
					return fmt.Errorf("Auth0 MCP server not found, please specify with --server-path")
				}
			}

			// Find node path if not provided
			if nodePath == "" {
				var err error
				nodePath, err = getNodePath()
				if err != nil {
					return fmt.Errorf("failed to find Node.js: %w", err)
				}
				cli.renderer.Infof("Using Node.js at: %s", nodePath)
			}

			// Find Claude config path if not provided
			if configPath == "" {
				var err error
				claudeConfigDir, err := getClaudeConfigDir()
				if err != nil {
					return fmt.Errorf("failed to determine Claude configuration directory: %w", err)
				}
				configPath = filepath.Join(claudeConfigDir, "claude_desktop_config.json")
			}

			// Update Claude Desktop config
			cli.renderer.Infof("Updating Claude Desktop configuration...")
			if err := updateClaudeConfig(configPath, tenant.Domain, token, nodePath, serverPath); err != nil {
				return fmt.Errorf("failed to update Claude Desktop configuration: %w", err)
			}

			cli.renderer.Infof("✅ Auth0 MCP server has been successfully configured for Claude Desktop")
			cli.renderer.Infof("Please restart Claude Desktop for the changes to take effect")
			
			// Show user instructions
			cli.renderer.Infof("")
			cli.renderer.Infof("To use Auth0 tools in Claude Desktop:")
			cli.renderer.Infof("1. Restart Claude Desktop")
			cli.renderer.Infof("2. Create a new conversation")
			cli.renderer.Infof("3. Try asking questions about your Auth0 tenant")
			cli.renderer.Infof("")
			cli.renderer.Infof("If you change your Auth0 tenant or your token expires, run:")
			cli.renderer.Infof("auth0 login")
			cli.renderer.Infof("auth0 mcp init")
			
			return nil
		},
	}

	cmd.Flags().StringVar(&nodePath, "node-path", "", "Path to Node.js executable")
	cmd.Flags().StringVar(&serverPath, "server-path", "", "Path to Auth0 MCP server (index.js)")
	cmd.Flags().StringVar(&configPath, "config-path", "", "Path to Claude Desktop configuration file")

	return cmd
}

// Update Claude Desktop config file
func updateClaudeConfig(configPath, domain, token, nodePath, serverPath string) error {
	// Debug info
	fmt.Printf("DEBUG: token value type: %T, length: %d\n", token, len(token))
	if len(token) > 10 {
		fmt.Printf("DEBUG: token preview: %s...%s\n", token[:5], token[len(token)-5:])
	} else if len(token) > 0 {
		fmt.Printf("DEBUG: token value: %s\n", token)
	} else {
		fmt.Printf("DEBUG: token is empty\n")
	}
	
	// Create config object
	config := claudeDesktopConfig{
		MCPServers: make(map[string]claudeMCPServer),
	}

	// Read existing config if it exists
	configExists := false
	if _, err := os.Stat(configPath); err == nil {
		configExists = true
		configData, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read Claude Desktop config: %w", err)
		}

		if err := json.Unmarshal(configData, &config); err != nil {
			return fmt.Errorf("failed to parse Claude Desktop config: %w", err)
		}
	}

	// Get the absolute path to the auth0-cli binary
	cliPath := filepath.Dir(os.Args[0])
	absCliPath, err := filepath.Abs(cliPath)
	if err != nil {
		fmt.Printf("Warning: Could not get absolute path for CLI directory: %v\n", err)
		absCliPath = cliPath
	}
	
	// Full path to the auth0 executable
	executableName := "auth0"
	if runtime.GOOS == "windows" {
		executableName = "auth0.exe"
	}
	fullCliPath := filepath.Join(absCliPath, executableName)
	
	// Check if the executable exists
	if _, err := os.Stat(fullCliPath); err != nil {
		fmt.Printf("Warning: auth0-cli executable not found at %s: %v\n", fullCliPath, err)
	} else {
		fmt.Printf("auth0-cli executable found at: %s\n", fullCliPath)
	}

	// Add Auth0 MCP server
	config.MCPServers["auth0"] = claudeMCPServer{
		Command: nodePath,
		Args:    []string{serverPath, "run", domain},
		Capabilities: []string{"tools"},
		Env: map[string]string{
			"DEBUG": "auth0-mcp:*",
			"AUTH0_DOMAIN": domain,
			// Include the full path to the CLI binary in the environment
			"AUTH0_CLI_PATH": fullCliPath,
			// Make sure the CLI directory is in the PATH
			"PATH": "/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:" + absCliPath,
		},
	}

	// Write config back
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to generate config JSON: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	if !configExists {
		fmt.Printf("Created Claude Desktop config file at: %s\n", configPath)
	} else {
		fmt.Printf("Updated Claude Desktop config file at: %s\n", configPath)
	}

	return nil
}

// Helper function to get Claude Desktop config directory
func getClaudeConfigDir() (string, error) {
	var configDir string

	switch runtime.GOOS {
	case "darwin":
		// macOS
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to find home directory: %w", err)
		}
		configDir = filepath.Join(home, "Library", "Application Support", "Claude")
	case "windows":
		// Windows
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		configDir = filepath.Join(appData, "Claude")
	case "linux":
		// Linux
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to find home directory: %w", err)
		}
		configDir = filepath.Join(home, ".config", "Claude")
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return configDir, nil
}

// Helper function to find Node.js path
func getNodePath() (string, error) {
	// Try to find node in PATH
	path, err := exec.LookPath("node")
	if err == nil {
		return path, nil
	}

	// Common node installation locations
	commonLocations := []string{
		"/usr/local/bin/node",
		"/usr/bin/node",
		"C:\\Program Files\\nodejs\\node.exe",
		"C:\\Program Files (x86)\\nodejs\\node.exe",
	}

	for _, loc := range commonLocations {
		if _, err := os.Stat(loc); err == nil {
			return loc, nil
		}
	}

	return "", fmt.Errorf("could not find Node.js executable, please specify with --node-path")
} 