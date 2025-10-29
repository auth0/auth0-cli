package cli

import (
	"context"
	"fmt"
	"log"
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
)

func aculDevCmd(cli *cli) *cobra.Command {
	var projectDir, port string
	var screenDirs []string

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Start development mode for ACUL project with automatic building and asset watching.",
		Long: `Start development mode for an ACUL project. This command:
- Runs 'npm run build' to build the project initially
- Watches the dist directory for asset changes
- Automatically patches screen assets when new builds are created
- Supports both single screen development and all screens

The project directory must contain package.json with a build script.
You need to run your own build process (e.g., npm run build, npm run screen <name>) 
to generate new assets that will be automatically detected and patched.`,
		Example: `  auth0 acul dev
  auth0 acul dev --dir ./my_acul_project
  auth0 acul dev --screen login-id --port 3000
  auth0 acul dev -d ./project -s login-id -p 8080`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAculDev(cmd.Context(), cli, projectDir, port, screenDirs)
		},
	}

	projectDirFlag.RegisterString(cmd, &projectDir, "")
	screenDevFlag.RegisterStringSlice(cmd, &screenDirs, nil)
	portFlag.RegisterString(cmd, &port, "8080")

	return cmd
}

func runAculDev(ctx context.Context, cli *cli, projectDir, port string, screenDirs []string) error {
	// Default to current directory
	if projectDir == "" {
		projectDir = "."
	}

	// Validate project structure
	if err := validateAculProject(projectDir); err != nil {
		return fmt.Errorf("invalid ACUL project: %w", err)
	}

	log.Printf("🚀 Starting ACUL development mode for project in %s", projectDir)

	// Initial build
	log.Println("🔨 Running initial build...")
	if err := buildProject(projectDir); err != nil {
		return fmt.Errorf("initial build failed: %w", err)
	}

	// Start asset watching and patching using existing logic
	log.Println("👀 Starting asset watcher...")
	log.Println("💡 Run 'npm run build' or 'npm run screen <name>' to generate new assets that will be automatically patched")

	//ToDO: Add log that says: Host your own server to serve the built assets in the same port.(Ex: like using 'npx serve dist -l <port>')

	assetsURL := fmt.Sprintf("http://localhost:%s", port)
	distPath := filepath.Join(projectDir, "dist")

	log.Printf("🌐 Assets URL: %s", assetsURL)
	log.Printf("👀 Watching screens: %v", screenDirs)

	// Reuse the existing watchAndPatch function
	return watchAndPatch(ctx, cli, assetsURL, distPath, screenDirs)
}

func validateAculProject(projectDir string) error {
	// Check for package.json
	packagePath := filepath.Join(projectDir, "package.json")
	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return fmt.Errorf("package.json not found. This doesn't appear to be a valid ACUL project")
	}

	// Check for src directory (typical for ACUL projects)
	srcPath := filepath.Join(projectDir, "src")
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("src directory not found. This doesn't appear to be a valid ACUL project structure")
	}

	return nil
}

func buildProject(projectDir string) error {
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	log.Println("✅ Build completed successfully")
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
				log.Printf("Screen directory %q not found in dist/assets: %v", screen, err)
				continue
			}
			screensToWatch = append(screensToWatch, screen)
		}
	}

	if err := watcher.Add(distPath); err != nil {
		log.Printf("Failed to watch %q: %v", distPath, err)
	} else {
		log.Printf("👀 Watching: %d screen(s): %v", len(screensToWatch), screensToWatch)
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
					log.Println("⏱️ Ignoring event due to debounce window")
					continue
				}
				lastProcessTime = now

				time.Sleep(500 * time.Millisecond) // short delay to let writes settle
				log.Println("📦 Change detected in assets folder. Rebuilding and patching assets...")

				// Patch the assets
				patchAssets(ctx, cli, distPath, assetsURL, screensToWatch, lastHeadTags)
			}

		case err := <-watcher.Errors:
			log.Printf("⚠️ Watcher error: %v", err)

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
				log.Printf("🔁 Skipping patch for '%s' — headTags unchanged", screen)
				return
			}

			log.Printf("📦 Detected changes for screen '%s'", screen)
			lastHeadTags[screen] = headTags

			settings := &management.PromptRendering{
				HeadTags: headTags,
			}

			if err = cli.api.Prompt.UpdateRendering(ctx, management.PromptType(ScreenPromptMap[screen]), management.ScreenName(screen), settings); err != nil {
				errChan <- fmt.Errorf("failed to patch settings for %s: %w", screen, err)
				return
			}

			log.Printf("✅ Successfully patched screen '%s'", screen)
		}(screen)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		log.Println("Watcher error: ", err)
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
