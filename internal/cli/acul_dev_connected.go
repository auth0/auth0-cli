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

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/auth0/go-auth0/management"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

var (
	// New flags for acul dev command
	projectDirFlag = Flag{
		Name:       "Project Directory",
		LongForm:   "dir",
		ShortForm:  "d",
		Help:       "Path to the ACUL project directory (must contain package.json).",
		IsRequired: false,
	}

	screenDevFlag = Flag{
		Name:         "Screen",
		LongForm:     "screen",
		ShortForm:    "s",
		Help:         "Specific screen to develop and watch. If not provided, will watch all screens in the dist/assets folder.",
		IsRequired:   false,
		AlwaysPrompt: false,
	}

	portFlag = Flag{
		Name:       "Port",
		LongForm:   "port",
		ShortForm:  "p",
		Help:       "Port for the local development server (default: 8080).",
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

In normal mode, you need to run your own build process (e.g., npm run build, npm run screen <name>) 
to generate new assets that will be automatically detected and patched.

In connected mode (--connected), this command will:
- Update the advance rendering settings of the chosen screens in your Auth0 tenant
- Run initial build and ask you to host assets locally
- Optionally run build:watch in the background for continuous asset updates
- Watch and patch assets automatically when changes are detected

⚠️  Connected mode should only be used on stage/dev tenants, not production!`,
		Example: `  auth0 acul dev
  auth0 acul dev --dir ./my_acul_project
  auth0 acul dev --screen login-id --port 3000
  auth0 acul dev -d ./project -s login-id -p 8080
  auth0 acul dev --connected --screen login-id --port 8080`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAculDev(cmd.Context(), cli, projectDir, port, screenDirs, connected)
		},
	}

	projectDirFlag.RegisterString(cmd, &projectDir, ".")
	screenDevFlag.RegisterStringSlice(cmd, &screenDirs, nil)
	portFlag.RegisterString(cmd, &port, "8080")
	connectedFlag.RegisterBool(cmd, &connected, false)

	return cmd
}

func runAculDev(ctx context.Context, cli *cli, projectDir, port string, screenDirs []string, connected bool) error {
	// Validate project structure
	if err := validateAculProject(projectDir); err != nil {
		return fmt.Errorf("invalid ACUL project: %w", err)
	}

	if connected {
		return runConnectedMode(ctx, cli, projectDir, port, screenDirs)
	}

	return runNormalMode(projectDir, port, screenDirs)
}

func runNormalMode(projectDir, port string, screenDirs []string) error {
	fmt.Printf("🚀  Starting ACUL development mode for project in %s\n", projectDir)
	fmt.Printf("📋  Development server will typically be available at:  %s\n\n", fmt.Sprintf("http://localhost:%s", port))
	fmt.Println("💡 Make changes to your code and view the live changes as we have HMR enabled!")

	// Run npm run dev command
	//cmd := exec.Command("npm", "run", "dev", "--", "--port", port)
	//ToDo: change back to use cmd once run dev command gets supported
	cmd := exec.Command("npm", "run", "screen", screenDirs[0])
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// ToDo: update when changed to dev command
	fmt.Printf("🔄 Executing: %s\n", ansi.Cyan("npm run screen "))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run 'npm run dev': %w", err)
	}

	return nil
}

func runConnectedMode(ctx context.Context, cli *cli, projectDir, port string, screenDirs []string) error {
	// Show warning and ask for confirmation with highlighted text
	cli.renderer.Warnf("")
	cli.renderer.Warnf("⚠️  %s", ansi.Bold("🌟 CONNECTED MODE ENABLED 🌟"))
	cli.renderer.Warnf("")
	cli.renderer.Infof("📢 %s", ansi.Cyan("This connected mode updates the advanced rendering settings"))
	cli.renderer.Infof("   %s", ansi.Cyan("of the chosen set of screens in your Auth0 tenant."))
	cli.renderer.Warnf("")
	cli.renderer.Errorf("🚨 %s", ansi.Bold("IMPORTANT: Use this ONLY on stage and dev tenants, NOT on production!"))
	//ToDO: Highlight the reason: If used on prod tenants which leads to upgrade configs with the updated localHost served ASSETS URL and may lead the user to incur unexpected charges for the users on production tenants.

	cli.renderer.Warnf("")

	// // Give user time to read the warning
	// cli.renderer.Infof("📖 Please take a moment to read the above warning carefully...")
	// cli.renderer.Infof("   Press Enter to continue...")
	// fmt.Scanln() // Wait for user to press Enter

	// Ask for confirmation
	if confirmed := prompt.Confirm("Do you want to proceed with connected mode?"); !confirmed {
		cli.renderer.Warnf("❌ Connected mode cancelled.")
		return nil
	}

	cli.renderer.Infof("")
	cli.renderer.Infof("🚀 Starting ACUL connected development mode for project in %s", ansi.Green(projectDir))

	// Step 1: Do initial build
	cli.renderer.Infof("")
	cli.renderer.Infof("🔨 %s", ansi.Bold("Step 1: Running initial build..."))
	if err := buildProject(cli, projectDir); err != nil {
		return fmt.Errorf("initial build failed: %w", err)
	}

	// Step 2: Ask user to host assets and get port confirmation
	cli.renderer.Infof("")
	cli.renderer.Infof("📡 %s", ansi.Bold("Step 2: Host your assets locally"))
	cli.renderer.Infof("Please either run the following command in a separate terminal to serve your assets or host someway on your own")
	cli.renderer.Infof("  %s", ansi.Cyan(fmt.Sprintf("npx serve dist -p %s --cors", port)))
	cli.renderer.Infof("")
	cli.renderer.Infof("This will serve your built assets at the specified port with CORS enabled.")

	assetsHosted := prompt.Confirm(fmt.Sprintf("Are you hosting the assets at http://localhost:%s?", port))
	if !assetsHosted {
		cli.renderer.Warnf("❌ Please host your assets first and run the command again.")
		return nil
	}

	// Step 3: Ask about build:watch
	cli.renderer.Infof("")
	cli.renderer.Infof("🔧 %s", ansi.Bold("Step 3: Continuous build watching (optional)"))
	cli.renderer.Infof("To ensure assets are updated with sample app code changes, you can:")
	cli.renderer.Infof("1. Manually re-run %s when you make changes, OR", ansi.Cyan("'npm run build'"))
	cli.renderer.Infof("2. Run %s in the background for continuous updates", ansi.Cyan("'npm run build:watch'"))
	cli.renderer.Infof("")
	cli.renderer.Infof("💡 Note: If you have auto-save enabled in your IDE, build:watch will rebuild")
	cli.renderer.Infof("   assets frequently (potentially every 15 seconds with changes).")

	runBuildWatch := prompt.Confirm("Would you like to run 'npm run build:watch' in the background?")

	var buildWatchCmd *exec.Cmd
	if runBuildWatch {
		cli.renderer.Infof("🔄 Starting %s in the background...", ansi.Cyan("'npm run build:watch'"))
		buildWatchCmd = exec.Command("npm", "run", "build:watch")
		buildWatchCmd.Dir = projectDir
		buildWatchCmd.Stdout = os.Stdout
		buildWatchCmd.Stderr = os.Stderr

		if err := buildWatchCmd.Start(); err != nil {
			cli.renderer.Warnf("⚠️  Failed to start build:watch: %v", err)
			cli.renderer.Infof("You can manually run %s when you make changes.", ansi.Cyan("'npm run build'"))
		} else {
			cli.renderer.Infof("✅ Build watch started successfully")
			// Ensure the process is killed when the main process exits
			defer func() {
				if buildWatchCmd.Process != nil {
					buildWatchCmd.Process.Kill()
				}
			}()
		}
	}

	// Step 4: Start watching and patching
	cli.renderer.Infof("")
	cli.renderer.Infof("👀 %s", ansi.Bold("Step 4: Starting asset watcher and patching..."))

	assetsURL := fmt.Sprintf("http://localhost:%s", port)
	distPath := filepath.Join(projectDir, "dist")

	cli.renderer.Infof("🌐 Assets URL: %s", ansi.Green(assetsURL))
	cli.renderer.Infof("👀 Watching screens: %v", screenDirs)
	cli.renderer.Infof("💡 Assets will be automatically patched when changes are detected in the dist folder")

	//ToDO: Give the user a hint to trigger the `auth0 test login` command to see the changes in action in their tenant's application.

	// Start watching and patching
	return watchAndPatch(ctx, cli, assetsURL, distPath, screenDirs)
}

func validateAculProject(projectDir string) error {
	// Check for package.json
	packagePath := filepath.Join(projectDir, "package.json")
	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return fmt.Errorf("package.json not found. This doesn't appear to be a valid ACUL project")
	}

	return nil
}

func buildProject(cli *cli, projectDir string) error {
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	cli.renderer.Infof("✅ Build completed successfully")
	return nil
}

func watchAndPatch(ctx context.Context, cli *cli, assetsURL, distPath string, screenDirs []string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	distAssetsPath := filepath.Join(distPath, "assets")
	var screensToWatch []string

	if len(screenDirs) == 1 && screenDirs[0] == "all" {
		dirs, err := os.ReadDir(distAssetsPath)
		if err != nil {
			return fmt.Errorf("failed to read assets dir: %w", err)
		}

		for _, d := range dirs {
			if d.IsDir() && d.Name() != "shared" {
				screensToWatch = append(screensToWatch, d.Name())
			}
		}
	} else {
		for _, screen := range screenDirs {
			path := filepath.Join(distAssetsPath, screen)
			_, err = os.Stat(path)
			if err != nil {
				cli.renderer.Warnf("Screen directory %q not found in dist/assets: %v", screen, err)
				continue
			}
			screensToWatch = append(screensToWatch, screen)
		}
	}

	if err := watcher.Add(distPath); err != nil {
		cli.renderer.Warnf("Failed to watch %q: %v", distPath, err)
	} else {
		cli.renderer.Infof("👀 Watching: %d screen(s): %v", len(screensToWatch), screensToWatch)
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

			// React to changes in dist/assets directory
			if strings.HasSuffix(event.Name, "assets") && event.Op&fsnotify.Create != 0 {
				now := time.Now()
				if now.Sub(lastProcessTime) < debounceWindow {
					cli.renderer.Infof("⏱️ Ignoring event due to debounce window")
					continue
				}
				lastProcessTime = now

				time.Sleep(500 * time.Millisecond) // short delay to let writes settle
				cli.renderer.Infof("📦 Change detected in assets folder. Rebuilding and patching assets...")

				// Patch the assets
				patchAssets(ctx, cli, distPath, assetsURL, screensToWatch, lastHeadTags)
			}

		case err := <-watcher.Errors:
			cli.renderer.Warnf("⚠️ Watcher error: %v", err)

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
				cli.renderer.Infof("🔁 Skipping patch for '%s' — headTags unchanged", screen)
				return
			}

			cli.renderer.Infof("📦 Detected changes for screen '%s'", ansi.Cyan(screen))
			lastHeadTags[screen] = headTags

			settings := &management.PromptRendering{
				HeadTags: headTags,
			}

			if err = cli.api.Prompt.UpdateRendering(ctx, management.PromptType(ScreenPromptMap[screen]), management.ScreenName(screen), settings); err != nil {
				errChan <- fmt.Errorf("failed to patch settings for %s: %w", screen, err)
				return
			}

			cli.renderer.Infof("✅ Successfully patched screen '%s'", ansi.Green(screen))
		}(screen)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		cli.renderer.Errorf("⚠️ Watcher error: %v", err)
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
			continue // skip on error
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			subDir := filepath.Base(dir)
			if subDir == "assets" {
				subDir = "" // root-level main-*.js
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
