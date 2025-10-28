package cli

import (
	"context"
	"fmt"
	"log"
	"os"
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
	watchFolder = Flag{
		Name:       "Watch Folder",
		LongForm:   "watch-folder",
		ShortForm:  "w",
		Help:       "Folder to watch for new builds. CLI will watch for changes in the folder and automatically update the assets.",
		IsRequired: true,
	}

	assetURL = Flag{
		Name:       "Assets URL",
		LongForm:   "assets-url",
		ShortForm:  "u",
		Help:       "Base URL for serving dist assets (e.g., http://localhost:5173).",
		IsRequired: true,
	}

	screensFlag1 = Flag{
		Name:         "screen",
		LongForm:     "screens",
		ShortForm:    "s",
		Help:         "watching screens",
		IsRequired:   true,
		AlwaysPrompt: true,
	}
)

func newUpdateAssetsCmd(cli *cli) *cobra.Command {
	var watchFolders, assetsURL string
	var screens []string

	cmd := &cobra.Command{
		Use:   "watch-assets",
		Short: "Watch the dist folder and patch screen assets. You can watch all screens or one or more specific screens.",
		Example: `  auth0 universal-login watch-assets --screens login-id,login,signup,email-identifier-challenge,login-passwordless-email-code --watch-folder "/dist" --assets-url "http://localhost:8080"
  auth0 ul watch-assets --screens all -w "/dist" -u "http://localhost:8080"
  auth0 ul watch-assets --screen login-id --watch-folder "/dist"" --assets-url "http://localhost:8080"
  auth0 ul switch -p login-id -s login-id -r standard`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return watchAndPatch(context.Background(), cli, assetsURL, watchFolders, screens)
		},
	}

	screensFlag1.RegisterStringSlice(cmd, &screens, nil)
	watchFolder.RegisterString(cmd, &watchFolders, "")
	assetURL.RegisterString(cmd, &assetsURL, "")

	return cmd
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
			info, err := os.Stat(path)
			if err != nil {
				log.Printf("Screen directory %q not found in dist/assets: %v", screen, err)
				continue
			}
			if !info.IsDir() {
				log.Printf("Screen path %q exists but is not a directory", path)
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

			if strings.HasSuffix(event.Name, "assets") && event.Op&(fsnotify.Create) != 0 {
				now := time.Now()
				if now.Sub(lastProcessTime) < debounceWindow {
					log.Println("⏱️ Ignoring event due to debounce window")
					continue
				}
				lastProcessTime = now

				time.Sleep(500 * time.Millisecond) // short delay to let writes settle
				log.Println("📦 Change detected in assets folder. Rebuilding and patching...")

				var wg sync.WaitGroup
				errChan := make(chan error, len(screensToWatch))

				for _, screen := range screensToWatch {
					wg.Add(1)

					go func(screen string) {
						defer wg.Done()

						headTags, err := buildHeadTagsFromDirs(filepath.Dir(distAssetsPath), assetsURL, screen)
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

						var settings = &management.PromptRendering{
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

				for err = range errChan {
					log.Println(err)
				}
			}

		case err = <-watcher.Errors:
			log.Println("Watcher error: ", err)

		case <-ctx.Done():
			return ctx.Err()
		}
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
