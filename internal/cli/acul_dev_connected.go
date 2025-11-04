package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
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
		return runConnectedMode(cmd.Context(), cli, projectDir, port, screenDirs)
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
	fmt.Println(ansi.Bold("🚀 Starting ") + ansi.Cyan("ACUL Dev Mode"))

	fmt.Printf("📂 Project: %s\n", ansi.Yellow(projectDir))

	fmt.Printf("🖥️  Server: %s\n", ansi.Green(fmt.Sprintf("http://localhost:%s", "3000")))
	fmt.Println("💡 " + ansi.Italic("Edit your code and see live changes instantly (HMR enabled)"))

	screen := screenDirs[0]

	// ToDo: change back to use cmd once run dev command gets supported. Run npm run dev command.
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

func runConnectedMode(ctx context.Context, cli *cli, projectDir, port string, screenDirs []string) error {
	if confirmed := showConnectedModeInformation(); !confirmed {
		fmt.Println(ansi.Red("❌ Connected mode cancelled."))
		return nil
	}

	fmt.Println("")
	fmt.Println("⚠️  " + ansi.Bold(ansi.Yellow("🌟 CONNECTED MODE ENABLED 🌟")))
	fmt.Println("")
	fmt.Println("🚀 " + ansi.Green(fmt.Sprintf("ACUL connected dev mode started for %s", projectDir)))

	// Determine screens to watch early after build.
	screensToWatch, err := getScreensToWatch(cli, projectDir, screenDirs)
	if err != nil {
		return fmt.Errorf("failed to determine screens to watch: %w", err)
	}

	fmt.Println("")
	fmt.Println("🔨 " + ansi.Bold(ansi.Blue("Step 1: Running initial build...")))
	if err := buildProject(cli, projectDir); err != nil {
		return fmt.Errorf("initial build failed: %w", err)
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
			defer func() {
				if serveCmd.Process != nil {
					serveCmd.Process.Kill()
				}
			}()
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
		fmt.Println("🔄 " + ansi.Cyan("Starting 'npm run build:watch' in the background..."))
		buildWatchCmd = exec.Command("npm", "run", "build:watch")
		buildWatchCmd.Dir = projectDir

		// Only show command output if debug mode is enabled.
		if cli.debug {
			buildWatchCmd.Stdout = os.Stdout
			buildWatchCmd.Stderr = os.Stderr
		}

		if err := buildWatchCmd.Start(); err != nil {
			fmt.Println("⚠️  " + ansi.Yellow("Failed to start build:watch: ") + ansi.Bold(err.Error()))
			fmt.Println("    You can manually run " + ansi.Cyan("'npm run build'") + " when changes are made.")
		} else {
			fmt.Println("✅ " + ansi.Green("Build watch started successfully"))
			defer func() {
				if buildWatchCmd.Process != nil {
					buildWatchCmd.Process.Kill()
				}
			}()
		}
	}

	fmt.Println("")
	fmt.Println("👀 " + ansi.Bold(ansi.Blue("Step 4: Starting asset watcher and patching...")))

	distPath := filepath.Join(projectDir, "dist")

	fmt.Println("🌐 Assets URL: " + ansi.Green(assetsURL))
	fmt.Println("👀 Watching screens: " + ansi.Cyan(strings.Join(screensToWatch, ", ")))
	fmt.Println("💡 " + ansi.Green("Assets will be patched automatically when changes are detected in the dist folder"))
	fmt.Println("")
	fmt.Println("🧪 " + ansi.Bold(ansi.Magenta("Tip: Run 'auth0 test login' to see your changes in action!")))

	return watchAndPatch(ctx, cli, assetsURL, distPath, screensToWatch)
}

func validateAculProject(projectDir string) error {
	packagePath := filepath.Join(projectDir, "package.json")
	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return fmt.Errorf("package.json not found. This doesn't appear to be a valid ACUL project")
	}
	return nil
}

// getScreensToWatch determines which screens to watch based on the provided screenDirs and available screens in the project.
func getScreensToWatch(cli *cli, projectDir string, screenDirs []string) ([]string, error) {
	distAssetsPath := filepath.Join(projectDir, "dist", "assets")

	var (
		screensToWatch []string
		screensInProj  []string
	)

	dirs, err := os.ReadDir(distAssetsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read assets dir: %w", err)
	}

	for _, d := range dirs {
		if d.IsDir() && d.Name() != "shared" {
			screensInProj = append(screensInProj, d.Name())
		}
	}

	if len(screensInProj) == 0 {
		return nil, fmt.Errorf("no valid screen directories found in dist/assets for the specified screens: %v", screenDirs)
	}

	switch {
	case len(screenDirs) == 0:
		screensToWatch, err = validateAndSelectScreens(cli, screensInProj, screenDirs)
		if err != nil {
			return nil, err
		}

	case len(screenDirs) == 1 && screenDirs[0] == "all":
		screensToWatch = screensInProj

	default:
		for _, screen := range screenDirs {
			path := filepath.Join(distAssetsPath, screen)
			if _, err := os.Stat(path); err != nil {
				fmt.Println("⚠️  " + ansi.Yellow(fmt.Sprintf("Screen directory '%s' not found in dist/assets: %v", screen, err)))
				continue
			}
			screensToWatch = append(screensToWatch, screen)
		}
	}

	return screensToWatch, nil
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

func watchAndPatch(ctx context.Context, cli *cli, assetsURL, distPath string, screensToWatch []string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := watcher.Add(distPath); err != nil {
		fmt.Println("⚠️  " + ansi.Yellow("Failed to watch ") + ansi.Bold(distPath) + ": " + err.Error())
	} else {
		fmt.Println("👀 Watching: " + ansi.Cyan(fmt.Sprintf("%d screen(s): %v", len(screensToWatch), screensToWatch)))
	}

	const debounceWindow = 5 * time.Second
	var lastProcessTime time.Time
	lastHeadTags := make(map[string][]interface{})

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// React to changes in dist/assets directory.
			if strings.HasSuffix(event.Name, "assets") && event.Op&fsnotify.Create != 0 {
				now := time.Now()
				if now.Sub(lastProcessTime) < debounceWindow {
					// Only show debounce message in debug mode.
					if cli.debug {
						cli.renderer.Infof("⏱️ %s", ansi.Yellow("Ignoring event due to debounce window"))
					}
					continue
				}
				lastProcessTime = now

				time.Sleep(500 * time.Millisecond) // Let writes settle.
				fmt.Println("📦 " + ansi.Cyan("Change detected in assets. Rebuilding and patching..."))

				patchAssets(ctx, cli, distPath, assetsURL, screensToWatch, lastHeadTags)
			}

		case err := <-watcher.Errors:
			fmt.Println("⚠️  " + ansi.Yellow("Watcher error: ") + ansi.Bold(err.Error()))

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func patchAssets(ctx context.Context, cli *cli, distPath, assetsURL string, screensToWatch []string, lastHeadTags map[string][]interface{}) {
	var wg sync.WaitGroup
	errChan := make(chan error, len(screensToWatch))

	for _, screen := range screensToWatch {
		wg.Add(1)
		go func(screen string) {
			defer wg.Done()

			headTags, err := buildHeadTagsFromDirs(distPath, assetsURL, screen)
			if err != nil {
				errChan <- fmt.Errorf("failed to build headTags for %s: %w", screen, err)
				return
			}

			if reflect.DeepEqual(lastHeadTags[screen], headTags) {
				fmt.Println("🔁 " + ansi.Cyan(fmt.Sprintf("Skipping patch for '%s' — headTags unchanged", screen)))
				return
			}

			if cli.debug {
				fmt.Println("📦 " + ansi.Cyan(fmt.Sprintf("Detected changes for '%s'", screen)))
			}
			lastHeadTags[screen] = headTags

			settings := &management.PromptRendering{
				RenderingMode: &management.RenderingModeAdvanced,
				HeadTags:      headTags,
			}

			if err = cli.api.Prompt.UpdateRendering(ctx, management.PromptType(ScreenPromptMap[screen]), management.ScreenName(screen), settings); err != nil {
				errChan <- fmt.Errorf("failed to patch settings for %s: %w", screen, err)
				return
			}

			fmt.Println("✅ " + ansi.Green(fmt.Sprintf("Successfully patched screen '%s'", screen)))
		}(screen)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		fmt.Println("⚠️  " + ansi.Yellow("Patch error: ") + ansi.Bold(err.Error()))
	}
}

func buildHeadTagsFromDirs(distPath, assetsURL, screen string) ([]interface{}, error) {
	var tags []interface{}
	screenPath := filepath.Join(distPath, "assets", screen)
	sharedPath := filepath.Join(distPath, "assets", "shared")
	mainPath := filepath.Join(distPath, "assets")
	sources := []string{sharedPath, screenPath, mainPath}

	for _, dir := range sources {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			subDir := filepath.Base(dir)
			if subDir == "assets" {
				subDir = ""
			}
			src := fmt.Sprintf("%s/assets/%s%s", assetsURL, subDir, name)
			if subDir != "" {
				src = fmt.Sprintf("%s/assets/%s/%s", assetsURL, subDir, name)
			}
			ext := filepath.Ext(name)

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
	return tags, nil
}
