package cli

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/auth0/go-auth0/management"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	projectDirFlag = Flag{
		Name:       "Project Directory",
		LongForm:   "dir",
		ShortForm:  "d",
		Help:       "Path to the ACUL project directory (must contain package.json).",
		IsRequired: false,
	}
	screenDevFlag = Flag{
		Name:         "Screens",
		LongForm:     "screens",
		ShortForm:    "s",
		Help:         "Specific screens to develop and watch.",
		IsRequired:   false,
		AlwaysPrompt: false,
	}

	portFlag = Flag{
		Name:       "Port",
		LongForm:   "port",
		ShortForm:  "p",
		Help:       "Port for the local development server.",
		IsRequired: false,
	}

	connectedFlag = Flag{
		Name:       "Connected",
		LongForm:   "connected",
		ShortForm:  "c",
		Help:       "Enable connected mode to update advance rendering settings of Auth0 tenant. Use only on stage/dev tenants.",
		IsRequired: false,
	}
)

func aculDevCmd(cli *cli) *cobra.Command {
	var projectDir, port string
	var screenDirs []string
	var connected bool

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Start development mode for ACUL project with automatic building and asset watching.",
		Long: `Start development mode for an ACUL project. This command:
- Runs 'npm run build' to build the project initially
- Watches the dist directory for asset changes
- Automatically patches screen assets when new builds are created
- Supports both single screen development and all screens

The project directory must contain package.json with a build script.

DEV MODE (default):
- Requires: --port flag for the local development server
- Runs your build process (e.g., npm run screen <name>) for HMR development

CONNECTED MODE (--connected):
- Requires: --screens flag to specify screens to patch in Auth0 tenant  
- Updates advance rendering settings of the chosen screens in your Auth0 tenant
- Runs initial build and expects you to host assets locally
- Optionally runs build:watch in the background for continuous asset updates
- Watches and patches assets automatically when changes are detected

⚠️  Connected mode should only be used on stage/dev tenants, not production!`,
		Example: `  # Dev mode
  auth0 acul dev --port 55444
  auth0 acul dev -p 55444 --dir ./my_project
  
  # Connected mode
  auth0 acul dev --connected
  auth0 acul dev --connected --debug --dir ./my_project
  auth0 acul dev --connected --screens all
  auth0 acul dev -c --dir ./my_project
  auth0 acul dev --connected --screens login-id
  auth0 acul dev -c -s login-id,signup`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureACULPrerequisites(cmd.Context(), cli.api); err != nil {
				return err
			}

			checkNodeVersion(cli)

			pwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %v", err)
			}

			if projectDir == "" {
				err = projectDirFlag.Ask(cmd, &projectDir, &pwd)
				if err != nil {
					return err
				}
			} else {
				fmt.Printf("📂 Project: %s\n", ansi.Yellow(projectDir))
			}

			if err = validateAculProject(projectDir); err != nil {
				return fmt.Errorf("invalid ACUL project: %w", err)
			}

			if connected {
				if confirmed := showConnectedModeInformation(); !confirmed {
					fmt.Println(ansi.Red("Connected mode cancelled."))
					return nil
				}

				fmt.Println("")
				fmt.Println("⚠️  " + ansi.Bold(ansi.Yellow("🌟 CONNECTED MODE ENABLED 🌟")))
				fmt.Println("")

				screensToWatch, err := selectScreensSimple(cli, projectDir, screenDirs)
				if err != nil {
					return fmt.Errorf("failed to determine screens to watch: %w", err)
				}

				return runConnectedMode(cmd.Context(), cli, projectDir, port, screensToWatch)
			}

			if port == "" {
				err = portFlag.Ask(cmd, &port, auth0.String("55444"))
				if err != nil {
					return err
				}
			} else {
				fmt.Printf("🖥️ Server: %s\n", ansi.Cyan(fmt.Sprintf("http://localhost:%s", port)))
			}
			return runNormalMode(cli, projectDir, port)
		},
	}

	projectDirFlag.RegisterString(cmd, &projectDir, "")
	screenDevFlag.RegisterStringSlice(cmd, &screenDirs, nil)
	portFlag.RegisterString(cmd, &port, "")
	connectedFlag.RegisterBool(cmd, &connected, false)

	return cmd
}

func runNormalMode(cli *cli, projectDir, port string) error {
	if !isPortFree(port) {
		return fmt.Errorf("port %s is already in use; please free it or choose another port", port)
	}

	// 1. Set up the command.
	cmd := exec.Command("npm", "run", "dev", "--", "--port", port)
	cmd.Dir = projectDir

	// 2. Set up output pipes/redirection.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to capture stdout: %w", err)
	}

	if cli.debug {
		cmd.Stderr = os.Stderr
		fmt.Println("\n🔄 Executing:", ansi.Cyan("npm run dev -- --port "+port))
	}

	// 3. Start the command asynchronously.
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start 'npm run dev -- --port %s': %w", port, err)
	}

	// 4. Print the success/info logs immediately after starting the server process.
	server := fmt.Sprintf("http://localhost:%s", port)

	// 5. Wait for the command to exit and handle intentional stops (Ctrl+C).
	readyChan := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			if strings.Contains(line, "Local:") && strings.Contains(line, "http") {
				close(readyChan)
				return
			}
		}
	}()

	select {
	case <-readyChan:
		fmt.Println("💡 " + ansi.Italic("Make changes to your code and view the live changes as we have HMR enabled!"))
		_ = browser.OpenURL(server)

	case <-time.After(20 * time.Second):
		fmt.Println("⏳ Dev server is taking longer than expected to start...")
	}

	if err = cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			fmt.Println(ansi.Bold("\n👋 Server stopped intentionally (Ctrl+C)."))
			return nil
		}

		return fmt.Errorf("dev server exited with an error: %w", err)
	}

	fmt.Println(ansi.Bold("\n'npm run dev' finished gracefully."))
	return nil
}

func showConnectedModeInformation() bool {
	fmt.Println("")
	fmt.Println("📢 " + ansi.Bold(ansi.Cyan("Connected Mode Information")))
	fmt.Println("")
	fmt.Println("ℹ️  " + ansi.Cyan("This mode updates advanced rendering settings for selected screens in your Auth0 tenant."))
	fmt.Println("")
	fmt.Println("🚨 " + ansi.Bold(ansi.Red("IMPORTANT: Never use on production tenants!")))
	fmt.Println("    " + ansi.Yellow("• Production may break sessions or incur unexpected charges with local assets."))
	fmt.Println("    " + ansi.Yellow("• Use ONLY for dev/stage tenants."))

	fmt.Println("")
	fmt.Println("⚙️  " + ansi.Bold(ansi.Magenta("Technical Requirements:")))
	fmt.Println("    " + ansi.Cyan("• Requires sample apps with viteConfig.ts configured for asset building"))
	fmt.Println("    " + ansi.Cyan("• Assets must be built in the following structure:"))
	fmt.Println("      " + ansi.Green("	assets/<screens>/"))
	fmt.Println("      " + ansi.Green("	assets/<shared>/"))
	fmt.Println("      " + ansi.Green("	assets/<main.*.js>"))
	fmt.Println("")
	fmt.Println("🔄  " + ansi.Bold(ansi.Magenta("How it works:")))
	fmt.Println("    " + ansi.Cyan("• Combines files from screen-specific, shared, and main asset folders"))
	fmt.Println("    " + ansi.Cyan("• Makes API patch calls to update rendering settings for each specified screen"))
	fmt.Println("    " + ansi.Cyan("• Watches for changes and automatically re-patches when assets are rebuilt"))
	fmt.Println("")

	return prompt.Confirm("Proceed with connected mode?")
}

// isPortFree returns true if port is free (no TCP connection possible).
func isPortFree(port string) bool {
	addr := net.JoinHostPort("127.0.0.1", port)
	conn, err := net.DialTimeout("tcp", addr, 250*time.Millisecond)
	if err != nil {
		return true
	}

	conn.Close()
	// Dial succeeded -> something is listening -> not free.
	return false
}

func runConnectedMode(ctx context.Context, cli *cli, projectDir, port string, screensToWatch []string) error {
	fmt.Println("\n🚀 " + ansi.Green("ACUL connected dev mode started for: "+ansi.Cyan(projectDir)))

	// Step 1: Do initial build.
	fmt.Println("")
	fmt.Println("🔨 " + ansi.Bold(ansi.Blue("Step 1: Running initial build with 'npm run build'")))
	if err := buildProject(cli, projectDir); err != nil {
		return fmt.Errorf("initial build failed: %w", err)
	}

	// Always validate screens after build to ensure they have actual built assets.
	screensToWatch, err := validateScreensAfterBuild(projectDir, screensToWatch)
	if err != nil {
		return fmt.Errorf("screen validation failed after build: %w", err)
	}

	// Step 2: Ask user to host assets and get port confirmation.
	fmt.Println("")
	fmt.Println("📡 " + ansi.Bold(ansi.Blue("Step 2: Host your assets locally")))

	if port == "" {
		var portInput string

		portQuestion := prompt.TextInput("port", "Enter port to serve assets:", "Example: 55444", "55444", true)
		if err := prompt.AskOne(portQuestion, &portInput); err != nil {
			return fmt.Errorf("failed to get port: %w", err)
		}

		if _, err = strconv.Atoi(portInput); err != nil {
			return fmt.Errorf("invalid port number: %s", portInput)
		}

		port = portInput
	}

	if !isPortFree(port) {
		return fmt.Errorf("port %s is already in use; please free it or choose another port", port)
	}

	fmt.Println("💡 " + ansi.Yellow("Your assets must be served locally with CORS enabled."))

	runServe := prompt.Confirm(fmt.Sprintf("Would you like to host the assets by running 'npx serve dist -p %s --cors' in the background?", port))

	var serveStarted bool

	if runServe {
		fmt.Println("🚀 " + ansi.Cyan("Starting local server in the background..."))

		serveCmd := exec.Command("npx", "serve", "dist", "-p", port, "--cors")
		serveCmd.Dir = projectDir

		if cli.debug {
			serveCmd.Stdout = os.Stdout
			serveCmd.Stderr = os.Stderr
		}

		if err := serveCmd.Start(); err != nil {
			fmt.Println("⚠️  " + ansi.Yellow("Failed to start local server: ") + ansi.Bold(err.Error()))
			fmt.Println("    You can manually run " + ansi.Cyan(fmt.Sprintf("'npx serve dist -p %s --cors'", port)) + " in a separate terminal.")
		} else {
			serveStarted = true
			fmt.Println("✅ " + ansi.Green("Local server started successfully at ") +
				ansi.Cyan(fmt.Sprintf("http://localhost:%s", port)))
			time.Sleep(2 * time.Second) // Give server time to start.
		}
	} else {
		fmt.Println("Please either run the following command in a separate terminal to serve your assets or host someway on your own")
		fmt.Println("    " + ansi.Bold(ansi.Green(fmt.Sprintf("npx serve dist -p %s --cors", port))))
		fmt.Println("")
		fmt.Println("This will serve your built assets at the specified port with CORS enabled.")
	}

	assetsURL := fmt.Sprintf("http://localhost:%s", port)

	// Only ask confirmation if not started in background.
	if !serveStarted {
		assetsHosted := prompt.Confirm(fmt.Sprintf("Are your assets hosted and accessible at %s?", assetsURL))
		if !assetsHosted {
			cli.renderer.Warnf("❌ Please host your assets first and run the command again.")
			return nil
		}
	}

	// Step 3: Ask about build:watch.
	fmt.Println("")
	fmt.Println("🔧 " + ansi.Bold(ansi.Blue("Step 3: Continuous build watching (optional)")))
	fmt.Println("    " + ansi.Green("1. Manually run 'npm run build' after changes, OR"))
	fmt.Println("    " + ansi.Green("2. Run 'npm run build:watch' for continuous updates"))
	fmt.Println("")
	fmt.Println("💡 " + ansi.Yellow("If auto-save is enabled in your IDE, build:watch will rebuild frequently."))

	runBuildWatch := prompt.Confirm("Would you like to run 'npm run build:watch' in the background?")

	if runBuildWatch {
		fmt.Println("🚀 " + ansi.Cyan("Starting 'npm run build:watch' in the background..."))
		buildWatchCmd := exec.Command("npm", "run", "build:watch")
		buildWatchCmd.Dir = projectDir

		// Only show command output if debug mode is enabled.
		if cli.debug {
			buildWatchCmd.Stdout = os.Stdout
			buildWatchCmd.Stderr = os.Stderr
		}

		if err := buildWatchCmd.Start(); err != nil {
			fmt.Println("⚠️  " + ansi.Yellow("Failed to start build:watch: ") + ansi.Bold(err.Error()))
			fmt.Println("    You can manually run " + ansi.Cyan("'npm run build'") + " whenever you update your code.")
		} else {
			fmt.Println("✅ " + ansi.Green("Build watch started successfully"))
		}
	}

	fmt.Println("")
	fmt.Println("👀 " + ansi.Bold(ansi.Blue("Step 4: Start watching assets and auto-patching...")))

	distPath := filepath.Join(projectDir, "dist")

	fmt.Println("🌐 Assets URL: " + ansi.Green(assetsURL))
	fmt.Println("👀 Watching screens: " + ansi.Cyan(strings.Join(screensToWatch, ", ")))

	// Fetch original head tags before starting watcher.
	fmt.Println("💡 " + ansi.Yellow("Note: Your original rendering settings will be saved and can be restored on exit."))
	originalHeadTags, err := fetchOriginalHeadTags(ctx, cli, screensToWatch)
	if err != nil {
		fmt.Println("⚠️  " + ansi.Yellow(fmt.Sprintf("Could not fetch original settings: %v", err)))
		fmt.Println("    " + ansi.Yellow("Restoration will be skipped since no previous settings could be retrieved."))
		originalHeadTags = nil // Continue without restoration capability.
	} else {
		fmt.Println("✅ " + ansi.Green(fmt.Sprintf("Saved original settings for %d screen(s)", len(originalHeadTags))))
	}

	fmt.Println()
	fmt.Println(ansi.Magenta("💡 Tips:"))
	fmt.Println(ansi.Cyan("  • Assets in '/dist/assets' are continuously monitored and patched when changes occur."))
	fmt.Println(ansi.Cyan("  • Run 'auth0 test login' anytime to preview your changes in real-time."))
	fmt.Println(ansi.Cyan("  • Press Ctrl+C to stop. You'll be prompted to restore your original settings."))
	fmt.Println()

	return watchAndPatch(ctx, cli, assetsURL, distPath, screensToWatch, originalHeadTags)
}

func validateAculProject(projectDir string) error {
	packagePath := filepath.Join(projectDir, "package.json")
	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return fmt.Errorf("package.json not found. This doesn't appear to be a valid ACUL project")
	}
	return nil
}

func buildProject(cli *cli, projectDir string) error {
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = projectDir

	// Only show command output if debug mode is enabled.
	if cli.debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Println("✅ " + ansi.Green("Build completed successfully"))
	return nil
}

func selectScreensSimple(cli *cli, projectDir string, screenDirs []string) ([]string, error) {
	// 1. Screens provided via --screens flag.
	if len(screenDirs) > 0 {
		if len(screenDirs) == 1 && screenDirs[0] == "all" {
			cli.renderer.Infof(ansi.Cyan("📂  Selecting all screens from src/screens"))

			return getScreensFromSrcFolder(filepath.Join(projectDir, "src", "screens"))
		}

		return screenDirs, nil
	}

	// 2. No --screens flag: auto-detect from src/screens.
	srcScreensPath := filepath.Join(projectDir, "src", "screens")

	if availableScreens, err := getScreensFromSrcFolder(srcScreensPath); err == nil && len(availableScreens) > 0 {
		return validateAndSelectScreens(cli, availableScreens, nil, true)
	}

	return nil, fmt.Errorf(`no screens found in project.

Please either:
1. Specify screens using --screens flag: auth0 acul dev --connected --screens login-id,signup
2. Create a new ACUL project: auth0 acul init
3. Ensure your project has screens in src/screens/ folder`)
}

func validateScreensAfterBuild(projectDir string, selectedScreens []string) ([]string, error) {
	distAssetsPath := filepath.Join(projectDir, "dist", "assets")

	availableScreens, err := getScreensFromDistAssets(distAssetsPath)

	if err != nil {
		return nil, fmt.Errorf("failed to read available screens from dist/assets: %w", err)
	}

	if len(availableScreens) == 0 {
		return nil, fmt.Errorf("no valid screens found in dist/assets after build")
	}

	availableScreensMap := make(map[string]bool)

	for _, screen := range availableScreens {
		availableScreensMap[screen] = true
	}

	var validScreens, missingScreens []string

	for _, screen := range selectedScreens {
		if availableScreensMap[screen] {
			validScreens = append(validScreens, screen)
		} else {
			missingScreens = append(missingScreens, screen)
		}
	}

	if len(missingScreens) > 0 {
		return nil, fmt.Errorf("⚠️  Missing built assets for: %s", strings.Join(missingScreens, ", "))
	}

	if len(validScreens) == 0 {
		return nil, fmt.Errorf(
			"none of the selected screens were built. Available built screens: %s",
			strings.Join(availableScreens, ", "),
		)
	}

	return validScreens, nil
}

// getScreensFromDistAssets reads screen names from dist/assets folder.
func getScreensFromDistAssets(distAssetsPath string) ([]string, error) {
	if _, err := os.Stat(distAssetsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("dist/assets not found")
	}

	dirs, err := os.ReadDir(distAssetsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read dist/assets: %w", err)
	}

	var screens []string
	for _, d := range dirs {
		if d.IsDir() && d.Name() != "shared" {
			screens = append(screens, d.Name())
		}
	}

	return screens, nil
}

// getScreensFromSrcFolder reads screen names from src/screens folder.
func getScreensFromSrcFolder(srcScreensPath string) ([]string, error) {
	if _, err := os.Stat(srcScreensPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("src/screens not found")
	}

	entries, err := os.ReadDir(srcScreensPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read src/screens: %w", err)
	}

	var screens []string
	for _, entry := range entries {
		if entry.IsDir() {
			screens = append(screens, entry.Name())
		}
	}

	return screens, nil
}

// fetchOriginalHeadTags retrieves the current rendering settings for all screens before making changes.
func fetchOriginalHeadTags(ctx context.Context, cli *cli, screensToWatch []string) (map[string][]interface{}, error) {
	originalTags := make(map[string][]interface{})

	existing, err := cli.api.Prompt.ListRendering(ctx)
	if err != nil {
		return nil, err
	}

	promptRenderingMap := make(map[string]*management.PromptRendering, len(existing.PromptRenderings))
	for _, r := range existing.PromptRenderings {
		if r.Prompt != nil && r.Screen != nil {
			key := string(*r.Prompt) + "|" + string(*r.Screen)
			promptRenderingMap[key] = r
		}
	}

	// Collect only requested screens.
	for _, screen := range screensToWatch {
		promptType := management.PromptType(ScreenPromptMap[screen])
		screenType := management.ScreenName(screen)
		key := string(promptType) + "|" + string(screenType)

		if r := promptRenderingMap[key]; r != nil && r.HeadTags != nil {
			originalTags[screen] = r.HeadTags
		}
	}

	return originalTags, nil
}

func applyPromptRenderings(ctx context.Context, cli *cli, screenTagMap map[string][]interface{}, debugPrefix string) error {
	var updates []*management.PromptRendering
	for screen, headTags := range screenTagMap {
		p := management.PromptType(ScreenPromptMap[screen])
		s := management.ScreenName(screen)
		updates = append(updates, &management.PromptRendering{
			Prompt:        &p,
			Screen:        &s,
			RenderingMode: &management.RenderingModeAdvanced,
			HeadTags:      headTags,
		})
	}

	if len(updates) == 0 {
		return fmt.Errorf("no renderings to apply")
	}

	// Snapshot originals.
	existing, err := cli.api.Prompt.ListRendering(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch current renderings: %w", err)
	}

	promptRenderingMap := make(map[string]*management.PromptRendering, len(existing.PromptRenderings))
	for _, r := range existing.PromptRenderings {
		if r.Prompt != nil && r.Screen != nil {
			promptRenderingMap[string(*r.Prompt)+"|"+string(*r.Screen)] = r
		}
	}

	originals := make([]*management.PromptRendering, len(updates))
	for i, u := range updates {
		originals[i] = promptRenderingMap[string(*u.Prompt)+"|"+string(*u.Screen)]
	}

	const maxBatch = 20
	doBatchedPatch := func(list []*management.PromptRendering) error {
		return cli.api.Prompt.BulkUpdateRendering(ctx,
			&management.PromptRenderingBulkUpdate{PromptRenderings: list})
	}

	// --- Batching loop with rollback awareness ---.
	for i := 0; i < len(updates); i += maxBatch {
		end := i + maxBatch
		if end > len(updates) {
			end = len(updates)
		}

		if err := doBatchedPatch(updates[i:end]); err != nil {
			if debugPrefix == "Restoring" {
				return fmt.Errorf("restore failed: %w", err)
			}

			if cli.debug {
				cli.renderer.Errorf("%s batch %d-%d failed: %v", debugPrefix, i+1, end, err)
				cli.renderer.Infof("%s rollback starting for screens 1-%d...", debugPrefix, i)
			} else {
				// Removed the redundant log: cli.renderer.Errorf("patch failed; rolling back...").
				cli.renderer.Warnf("Patch failed. Attempting rollback...")
			}

			// Rollback all fully-applied previous batches.
			for r := 0; r < i; r += maxBatch {
				rEnd := r + maxBatch
				if rEnd > i {
					rEnd = i
				}

				if rbErr := doBatchedPatch(originals[r:rEnd]); rbErr != nil {
					if cli.debug {
						cli.renderer.Warnf("%s rollback failed for screens %d-%d: %v", debugPrefix, r+1, rEnd, rbErr)
					} else {
						cli.renderer.Warnf("Partial rollback failed for screens %d-%d.", r+1, rEnd)
					}
				} else {
					if cli.debug {
						cli.renderer.Infof("%s rollback restored screens %d-%d", debugPrefix, r+1, rEnd)
					}
				}
			}

			cli.renderer.Infof("Rollback complete for all applied batches.")

			if cli.debug {
				return fmt.Errorf("%s update failed at batch %d-%d: %w", debugPrefix, i+1, end, err)
			}

			return fmt.Errorf("%s failed: %w", debugPrefix, err)
		}
	}

	return nil
}

func watchAndPatch(ctx context.Context, cli *cli, assetsURL, distPath string, screensToWatch []string, originalHeadTags map[string][]interface{}) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file system watcher: %w", err)
	}
	defer watcher.Close()

	if err := watcher.Add(distPath); err != nil {
		return fmt.Errorf("failed to watch distribution path %s: %w", distPath, err)
	}

	fmt.Println("👀  Watching: " + ansi.Yellow(strings.Join(screensToWatch, ", ")))

	// First, stop any existing global signal handlers (from root.go).
	signal.Reset(os.Interrupt, syscall.SIGTERM)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	const debounceWindow = 5 * time.Second
	var (
		lastEventTime time.Time
		lastHeadTags  = make(map[string][]interface{})
	)

	cleanup := func() {
		time.Sleep(1 * time.Second)
		fmt.Fprintln(os.Stderr, ansi.Yellow("\nShutting down ACUL connected mode..."))
		if len(originalHeadTags) > 0 {
			restore := prompt.Confirm("Would you like to restore your original rendering settings?")

			if restore {
				fmt.Fprintln(os.Stderr, ansi.Cyan("Restoring original rendering settings..."))

				if err := applyPromptRenderings(ctx, cli, originalHeadTags, "Restoring"); err != nil {
					fmt.Fprintln(os.Stderr, ansi.Yellow(fmt.Sprintf("Restoration failed: %v", err)))
				} else {
					fmt.Fprintln(os.Stderr, ansi.Green(fmt.Sprintf("Successfully restored rendering settings for %d screen(s).", len(originalHeadTags))))
				}
			} else {
				fmt.Fprintln(os.Stderr, ansi.Yellow("Restoration skipped. The patched assets will continue to remain active in your Auth0 tenant."))
			}
		}

		fmt.Println()
		fmt.Fprintln(os.Stderr, ansi.Green("👋 ACUL connected mode stopped."))

		fmt.Fprintf(os.Stderr, "%s  Use %s to see all available commands\n\n",
			ansi.Yellow("💡 Tip:"), ansi.Cyan("auth0 acul --help"))
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Ignore non-asset events or irrelevant file ops.
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 || !strings.Contains(event.Name, "assets") {
				continue
			}

			now := time.Now()
			if now.Sub(lastEventTime) < debounceWindow {
				continue
			}
			lastEventTime = now

			time.Sleep(500 * time.Millisecond) // Let writes settle.

			newHeadTags := make(map[string][]interface{})
			changedScreens := make([]string, 0)

			for _, screen := range screensToWatch {
				headTags, err := buildHeadTagsFromDirs(distPath, assetsURL, screen)
				if err != nil {
					if cli.debug {
						cli.renderer.Warnf(ansi.Yellow(fmt.Sprintf("Skipping '%s': failed to build head tags: %v", screen, err)))
					}
					continue
				}

				// Compare with last known tags.
				if reflect.DeepEqual(lastHeadTags[screen], headTags) {
					continue
				}

				// Only record changed screens.
				newHeadTags[screen] = headTags
				changedScreens = append(changedScreens, screen)
			}

			if len(changedScreens) == 0 {
				if cli.debug {
					fmt.Println(ansi.Yellow("No effective asset changes detected, skipping patch."))
				}
				continue
			}

			if cli.debug {
				fmt.Println(ansi.Cyan(fmt.Sprintf("🔄 Changes detected in %d screen(s): %s", len(changedScreens),
					strings.Join(changedScreens, ", "))))
			} else {
				fmt.Println(ansi.Cyan("⚙️ Change detected, patching assets..."))
			}

			if err = applyPromptRenderings(ctx, cli, newHeadTags, "Patching"); err != nil {
				cli.renderer.Errorf("Patching assets failed: %v", err)
			} else {
				fmt.Println(ansi.Green("✅ Assets patched successfully!"))
				for screen, headTags := range newHeadTags {
					lastHeadTags[screen] = headTags
				}
			}

		case err := <-watcher.Errors:
			cli.renderer.Warnf("File watcher internal error: %v", err)

		case <-sigChan:
			cleanup()
			return nil

		case <-ctx.Done():
			cleanup()
			return ctx.Err()
		}
	}
}

// buildHeadTagsFromDirs collects <script> and <link> tags from shared, screen-specific, and common asset directories.
func buildHeadTagsFromDirs(distPath, assetsURL, screen string) ([]interface{}, error) {
	searchDirs := []string{
		filepath.Join(distPath, "assets", "shared"),
		filepath.Join(distPath, "assets", screen),
		filepath.Join(distPath, "assets"),
	}

	var tags []interface{}
	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if e.IsDir() {
				continue
			}

			ext := filepath.Ext(e.Name())
			subDir := filepath.Base(dir)
			if subDir == "assets" {
				subDir = ""
			}

			src := fmt.Sprintf("%s/assets", assetsURL)
			if subDir != "" {
				src = fmt.Sprintf("%s/%s", src, subDir)
			}
			src = fmt.Sprintf("%s/%s", src, e.Name())

			switch ext {
			case ".js":
				tags = append(tags, map[string]interface{}{
					"tag": "script",
					"attributes": map[string]interface{}{
						"src":   src,
						"defer": true,
						"type":  "module",
					},
				})
			case ".css":
				tags = append(tags, map[string]interface{}{
					"tag": "link",
					"attributes": map[string]interface{}{
						"href": src,
						"rel":  "stylesheet",
					},
				})
			}
		}
	}
	if len(tags) == 0 {
		return nil, fmt.Errorf("no .js or .css assets found for '%s'", screen)
	}
	return tags, nil
}
