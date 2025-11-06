package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/auth0/go-auth0/management"
	"github.com/fsnotify/fsnotify"
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
		LongForm:     "screen",
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
- Requires: --screen flag to specify screens to patch in Auth0 tenant  
- Updates advance rendering settings of the chosen screens in your Auth0 tenant
- Runs initial build and expects you to host assets locally
- Optionally runs build:watch in the background for continuous asset updates
- Watches and patches assets automatically when changes are detected

⚠️  Connected mode should only be used on stage/dev tenants, not production!`,
		Example: `  # Dev mode
  auth0 acul dev --port 3000
  auth0 acul dev -p 8080 --dir ./my_project
  
  # Connected mode
  auth0 acul dev --connected
  auth0 acul dev --connected --debug --dir ./my_project
  auth0 acul dev --connected --screen all
  auth0 acul dev -c --dir ./my_project
  auth0 acul dev --connected --screen login-id
  auth0 acul dev -c -s login-id,signup`,
		RunE: func(cmd *cobra.Command, args []string) error {
			pwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %v", err)
			}

			if projectDir == "" {
				err = projectDirFlag.Ask(cmd, &projectDir, &pwd)
				if err != nil {
					return err
				}
			}

			if err := validateAculProject(projectDir); err != nil {
				return fmt.Errorf("invalid ACUL project: %w", err)
			}

			return runAculDev(cmd, cli, projectDir, port, screenDirs, connected)
		},
	}

	projectDirFlag.RegisterString(cmd, &projectDir, "")
	screenDevFlag.RegisterStringSlice(cmd, &screenDirs, nil)
	portFlag.RegisterString(cmd, &port, "")
	connectedFlag.RegisterBool(cmd, &connected, false)

	return cmd
}

func runAculDev(cmd *cobra.Command, cli *cli, projectDir, port string, screenDirs []string, connected bool) error {
	if connected {
		if confirmed := showConnectedModeInformation(); !confirmed {
			fmt.Println(ansi.Red("❌ Connected mode cancelled."))
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
		err := portFlag.Ask(cmd, &projectDir, auth0.String("8080"))
		if err != nil {
			return err
		}
	}
	return runNormalMode(cli, projectDir, screenDirs)
}

// ToDo : use the port logic.
func runNormalMode(cli *cli, projectDir string, screenDirs []string) error {
	var screen string
	fmt.Println(ansi.Bold("🚀 Starting ") + ansi.Cyan("ACUL Dev Mode"))

	fmt.Printf("📂 Project: %s\n", ansi.Yellow(projectDir))

	fmt.Printf("🖥️  Server: %s\n", ansi.Green(fmt.Sprintf("http://localhost:%s", "3000")))
	fmt.Println("💡 " + ansi.Italic("Edit your code and see live changes instantly (HMR enabled)"))

	if len(screenDirs) == 0 {
		screen = "login-id"
		// ToDo: change back to use cmd once run dev command gets supported. Run npm run dev command.
		fmt.Println("Defaulting to running 'npm run screen login-id' for dev mode...")
	} else {
		screen = screenDirs[0]
		fmt.Println("Running 'npm run screen " + screen + "' for dev mode...")
	}

	cmd := exec.Command("npm", "run", "screen", screen)
	cmd.Dir = projectDir

	// Show output only in debug mode.
	if cli.debug {
		fmt.Println("\n🔄 Running:", ansi.Cyan(fmt.Sprintf("npm run screen %s", screen)))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("❌ failed to run 'npm run screen %s': %w", screen, err)
	}

	return nil
}

func showConnectedModeInformation() bool {
	fmt.Println("")
	fmt.Println("📢 " + ansi.Bold(ansi.Cyan("Connected Mode Information")))
	fmt.Println("")
	fmt.Println("ℹ️  " + ansi.Cyan("This mode updates advanced rendering settings for selected screens in your Auth0 tenant."))
	fmt.Println("🚨 " + ansi.Bold(ansi.Red("IMPORTANT: Never use on production tenants!")))
	fmt.Println("    " + ansi.Yellow("Production may break sessions or incur unexpected charges with local assets."))
	fmt.Println("    " + ansi.Yellow("Use ONLY for dev/stage tenants."))

	fmt.Println("")
	fmt.Println("⚙️  " + ansi.Bold(ansi.Magenta("Technical Requirements:")))
	fmt.Println("    " + ansi.Cyan("• Requires sample apps with viteConfig.ts configured for asset building"))
	fmt.Println("    " + ansi.Cyan("• Assets must be built in the following structure:"))
	fmt.Println("      " + ansi.Green("assets/<screens>/"))
	fmt.Println("      " + ansi.Green("assets/<shared>/"))
	fmt.Println("      " + ansi.Green("assets/<main.*.js>"))
	fmt.Println("")
	fmt.Println("🔄 " + ansi.Bold(ansi.Magenta("How it works:")))
	fmt.Println("    " + ansi.Cyan("• Combines files from screen-specific, shared, and main asset folders"))
	fmt.Println("    " + ansi.Cyan("• Makes API patch calls to update rendering settings for each specified screen"))
	fmt.Println("    " + ansi.Cyan("• Watches for changes and automatically re-patches when assets are rebuilt"))
	fmt.Println("")

	return prompt.Confirm("Proceed with connected mode?")
}

func runConnectedMode(ctx context.Context, cli *cli, projectDir, port string, screensToWatch []string) error {
	fmt.Println("🚀 " + ansi.Green(fmt.Sprintf("ACUL connected dev mode started for %s", projectDir)))

	fmt.Println("")
	fmt.Println("🔨 " + ansi.Bold(ansi.Blue("Step 1: Running initial build...")))
	if err := buildProject(cli, projectDir); err != nil {
		return fmt.Errorf("initial build failed: %w", err)
	}

	// Always validate screens after build to ensure they have actual built assets.
	screensToWatch, err := validateScreensAfterBuild(projectDir, screensToWatch)
	if err != nil {
		return fmt.Errorf("screen validation failed after build: %w", err)
	}

	fmt.Println("")
	fmt.Println("📡 " + ansi.Bold(ansi.Blue("Step 2: Host your assets locally")))

	if port == "" {
		var portInput string
		portQuestion := prompt.TextInput(
			"port",
			"Enter the port for serving assets:",
			"The port number where your assets will be hosted (e.g., 8080)",
			"8080",
			true,
		)
		if err := prompt.AskOne(portQuestion, &portInput); err != nil {
			return fmt.Errorf("failed to get port: %w", err)
		}
		port = portInput
	}

	fmt.Println("💡 " + ansi.Yellow("Your assets need to be served locally with CORS enabled."))

	runServe := prompt.Confirm(fmt.Sprintf("Would you like to host the assets by running 'npx serve dist -p %s --cors' in the background?", port))

	var (
		serveCmd     *exec.Cmd
		serveStarted bool
	)
	if runServe {
		fmt.Println("🚀 " + ansi.Cyan("Starting local server in the background..."))

		serveCmd = exec.Command("npx", "serve", "dist", "-p", port, "--cors")
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
		fmt.Println("📋 " + ansi.Cyan("Please host your assets manually using:"))
		fmt.Println("    " + ansi.Bold(ansi.Green(fmt.Sprintf("npx serve dist -p %s --cors", port))))
		fmt.Println("")
		fmt.Println("💡 " + ansi.Yellow("This will serve your built assets with CORS enabled."))
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

	fmt.Println("")
	fmt.Println("🔧 " + ansi.Bold(ansi.Blue("Step 3: Continuous build watching (optional)")))
	fmt.Println("    " + ansi.Green("1. Manually run 'npm run build' after changes, OR"))
	fmt.Println("    " + ansi.Green("2. Run 'npm run build:watch' for continuous updates"))
	fmt.Println("")
	fmt.Println("💡 " + ansi.Yellow("Note: If auto-save is enabled in your IDE, build:watch will rebuild frequently."))

	runBuildWatch := prompt.Confirm("Would you like to run 'npm run build:watch' in the background?")

	var buildWatchCmd *exec.Cmd
	if runBuildWatch {
		fmt.Println("🚀 " + ansi.Cyan("Starting 'npm run build:watch' in the background..."))
		buildWatchCmd = exec.Command("npm", "run", "build:watch")
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
	fmt.Println("👀 " + ansi.Bold(ansi.Blue("Step 4: Starting asset watcher and patching...")))

	distPath := filepath.Join(projectDir, "dist")

	fmt.Println("🌐 Assets URL: " + ansi.Green(assetsURL))
	fmt.Println("👀 Watching screens: " + ansi.Cyan(strings.Join(screensToWatch, ", ")))

	// Fetch original head tags before starting watcher.
	fmt.Println("💡 " + ansi.Cyan("Fetching original rendering settings for restoration on exit..."))
	originalHeadTags, err := fetchOriginalHeadTags(ctx, cli, screensToWatch)
	if err != nil {
		fmt.Println("⚠️  " + ansi.Yellow(fmt.Sprintf("Warning: Could not fetch original settings: %v", err)))
		fmt.Println("    " + ansi.Yellow("Original settings will not be restored on exit."))
		originalHeadTags = nil // Continue without restoration capability.
	} else {
		fmt.Println("✅ " + ansi.Green(fmt.Sprintf("Original settings saved for %d screen(s)", len(originalHeadTags))))
	}

	fmt.Println("")
	fmt.Println("💡 " + ansi.Green("Assets will be patched automatically when changes are detected in the dist folder"))
	fmt.Println("")
	fmt.Println(ansi.Bold(ansi.Magenta("Tip: Run 'auth0 test login' to see your changes in action!")))
	fmt.Println(ansi.Cyan("Press Ctrl+C to stop and restore original settings"))

	return watchAndPatch(ctx, cli, assetsURL, distPath, screensToWatch, buildWatchCmd, serveCmd, serveStarted, originalHeadTags)
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
	// 1. Screens provided via --screen flag.
	if len(screenDirs) > 0 {
		if len(screenDirs) == 1 && screenDirs[0] == "all" {
			cli.renderer.Infof(ansi.Cyan("📂  Selecting all screens from src/screens"))

			return getScreensFromSrcFolder(filepath.Join(projectDir, "src", "screens"))
		}

		cli.renderer.Infof(ansi.Cyan(fmt.Sprintf("📂  Using specified screens: %s", strings.Join(screenDirs, ", "))))

		return screenDirs, nil
	}

	// 2. No --screen flag: auto-detect from src/screens.
	srcScreensPath := filepath.Join(projectDir, "src", "screens")

	if availableScreens, err := getScreensFromSrcFolder(srcScreensPath); err == nil && len(availableScreens) > 0 {
		cli.renderer.Infof(ansi.Cyan(fmt.Sprintf("📂  Detected screens in src/screens: %s", strings.Join(availableScreens, ", "))))

		return validateAndSelectScreens(cli, availableScreens, nil)
	}

	return nil, fmt.Errorf(`no screens found in project.

Please either:
1. Specify screens using --screen flag: auth0 acul dev --connected --screen login-id,signup
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

	for _, screen := range screensToWatch {
		promptType := management.PromptType(ScreenPromptMap[screen])
		screenType := management.ScreenName(screen)

		rendering, err := cli.api.Prompt.ReadRendering(ctx, promptType, screenType)
		if err != nil {
			if cli.debug {
				fmt.Println("⚠️  " + ansi.Yellow(fmt.Sprintf("Could not fetch original settings for '%s': %v", screen, err)))
			}
			continue
		}

		if rendering != nil && rendering.HeadTags != nil {
			originalTags[screen] = rendering.HeadTags
			if cli.debug {
				fmt.Println("📥 " + ansi.Cyan(fmt.Sprintf("Saved original settings for '%s' (%d tags)", screen, len(rendering.HeadTags))))
			}
		}
	}

	return originalTags, nil
}

// restoreOriginalHeadTags restores the original rendering settings that were saved at startup.
func restoreOriginalHeadTags(ctx context.Context, cli *cli, originalHeadTags map[string][]interface{}) error {
	var renderings []*management.PromptRendering

	for screen, headTags := range originalHeadTags {
		promptType := management.PromptType(ScreenPromptMap[screen])
		screenType := management.ScreenName(screen)

		renderings = append(renderings, &management.PromptRendering{
			Prompt:        &promptType,
			Screen:        &screenType,
			RenderingMode: &management.RenderingModeAdvanced,
			HeadTags:      headTags,
		})

		if cli.debug {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("   🔄 Restoring '%s' with %d tags", screen, len(headTags)))
		}
	}

	if len(renderings) == 0 {
		return fmt.Errorf("no original settings to restore")
	}

	req := &management.PromptRenderingUpdateRequest{PromptRenderings: renderings}
	if err := cli.api.Prompt.BulkUpdateRendering(ctx, req); err != nil {
		return fmt.Errorf("bulk restore error: %w", err)
	}

	return nil
}

func watchAndPatch(ctx context.Context, cli *cli, assetsURL, distPath string, screensToWatch []string, buildWatchCmd, serveCmd *exec.Cmd, serveStarted bool, originalHeadTags map[string][]interface{}) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	if err := watcher.Add(distPath); err != nil {
		cli.renderer.Warnf("Failed to watch %s: %v", distPath, err)
	}

	cli.renderer.Infof("Watching: %s", strings.Join(screensToWatch, ", "))

	// Signal handling for graceful shutdown
	// First, stop any existing global signal handlers (from root.go)
	signal.Reset(os.Interrupt, syscall.SIGTERM)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan) // Clean up signal handler when function exits

	const debounceWindow = 5 * time.Second
	var lastEventTime time.Time
	lastHeadTags := make(map[string][]interface{})

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Trigger only on changes inside dist/assets/.
			if !strings.Contains(event.Name, "assets") {
				continue
			}

			now := time.Now()
			if now.Sub(lastEventTime) < debounceWindow {
				continue
			}
			lastEventTime = now

			time.Sleep(500 * time.Millisecond) // Let writes settle.
			cli.renderer.Warnf(ansi.Cyan("Change detected, patching assets..."))
			if err := patchAssets(ctx, cli, distPath, assetsURL, screensToWatch, lastHeadTags); err != nil {
				cli.renderer.Errorf("Patch failed: %v", err)
			}

		case err := <-watcher.Errors:
			cli.renderer.Warnf("Watcher error: %v", err)

		case <-sigChan:
			fmt.Fprintln(os.Stderr, "\nShutdown signal received, cleaning up...")

			// Restore original head tags if available
			if originalHeadTags != nil && len(originalHeadTags) > 0 {
				fmt.Fprintln(os.Stderr, "Restoring original settings...")

				if err := restoreOriginalHeadTags(ctx, cli, originalHeadTags); err != nil {
					fmt.Fprintf(os.Stderr, " WARN  Could not restore: %v\n", err)
				} else {
					fmt.Fprintf(os.Stderr, " INFO  Restored settings for %d screen(s)\n", len(originalHeadTags))
				}
			}

			// Stop background processes
			if buildWatchCmd != nil && buildWatchCmd.Process != nil {
				fmt.Fprintln(os.Stderr, "Stopping build watcher...")
				if err := buildWatchCmd.Process.Kill(); err != nil {
					fmt.Fprintf(os.Stderr, " Error stopping build watcher: %v\n", err)
				}
			}

			if serveCmd != nil && serveCmd.Process != nil && serveStarted {
				fmt.Fprintln(os.Stderr, "Stopping local server...")
				if err := serveCmd.Process.Kill(); err != nil {
					fmt.Fprintf(os.Stderr, "  Error stopping server: %v\n", err)
				}
			}

			fmt.Fprintln(os.Stderr, "\nACUL connected mode stopped. Goodbye!")

			watcher.Close()
			return nil

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func patchAssets(ctx context.Context, cli *cli, distPath, assetsURL string, screensToWatch []string, lastHeadTags map[string][]interface{}) error {
	var (
		renderings []*management.PromptRendering
		updated    []string
	)

	for _, screen := range screensToWatch {
		headTags, err := buildHeadTagsFromDirs(distPath, assetsURL, screen)
		if err != nil {
			if cli.debug {
				cli.renderer.Warnf(ansi.Yellow(fmt.Sprintf("Skipping '%s': %v", screen, err)))
			}
			continue
		}

		if reflect.DeepEqual(lastHeadTags[screen], headTags) {
			if cli.debug {
				cli.renderer.Warnf(ansi.Yellow(fmt.Sprintf("No changes detected for '%s'", screen)))
			}
			continue
		}
		lastHeadTags[screen] = headTags

		promptType := management.PromptType(ScreenPromptMap[screen])
		screenType := management.ScreenName(screen)

		renderings = append(renderings, &management.PromptRendering{
			Prompt:        &promptType,
			Screen:        &screenType,
			RenderingMode: &management.RenderingModeAdvanced,
			HeadTags:      headTags,
		})
		updated = append(updated, screen)
	}

	if len(renderings) == 0 {
		if cli.debug {
			cli.renderer.Warnf(ansi.Cyan("No screens to patch"))
		}
		return nil
	}

	req := &management.PromptRenderingUpdateRequest{PromptRenderings: renderings}
	if err := cli.api.Prompt.BulkUpdateRendering(ctx, req); err != nil {
		return fmt.Errorf("bulk patch error: %w", err)
	}

	cli.renderer.Infof(ansi.Green(fmt.Sprintf("✅  Patched %d screen(s): %s", len(updated), strings.Join(updated, ", "))))

	return nil
}

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
