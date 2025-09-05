package cli

import (
	"encoding/json"
	"fmt"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/auth0/auth0-cli/internal/utils"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Manifest struct {
	Templates map[string]Template `json:"templates"`
	Metadata  Metadata            `json:"metadata"`
}

type Template struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Framework       string   `json:"framework"`
	SDK             string   `json:"sdk"`
	BaseFiles       []string `json:"base_files"`
	BaseDirectories []string `json:"base_directories"`
	Screens         []Screen `json:"screens"`
}

type Screen struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

type Metadata struct {
	Version     string `json:"version"`
	Repository  string `json:"repository"`
	LastUpdated string `json:"last_updated"`
	Description string `json:"description"`
}

// raw GitHub base URL
const rawBaseURL = "https://raw.githubusercontent.com"

func main() {

}

func fetchManifest() (*Manifest, error) {
	// The URL to the raw JSON file in the repository.
	url := "https://raw.githubusercontent.com/auth0-samples/auth0-acul-samples/monorepo-sample/manifest.json"

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch manifest: received status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read manifest body: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest format: %w", err)
	}

	return &manifest, nil
}

// This logic goes inside your `RunE` function.
func aculInitCmd(c *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Args:  cobra.MaximumNArgs(1),
		Short: "Generate a new project from a template",
		Long:  `Generate a new project from a template.`,
		RunE: func(cmd *cobra.Command, args []string) error {

			manifest, err := fetchManifest()
			if err != nil {
				return err
			}

			// Step 2: select template
			var templateNames []string
			for k := range manifest.Templates {
				templateNames = append(templateNames, k)
			}

			var chosen string
			promptText := prompt.SelectInput("", "Select a template", "Chosen template(Todo)", utils.FetchKeys(manifest.Templates), "react-js", true)
			if err := prompt.AskOne(promptText, &chosen); err != nil {

			}

			// Step 3: select screens
			var screenOptions []string
			template := manifest.Templates[chosen]
			for _, s := range template.Screens {
				screenOptions = append(screenOptions, s.ID)
			}

			// Step 3: Let user select screens
			var selectedScreens []string
			if err := prompt.AskMultiSelect("Select screens to include:", &selectedScreens, screenOptions...); err != nil {
				return err
			}

			var targetRoot string
			if len(args) < 1 {
				targetRoot = "my_acul_proj"
			} else {
				targetRoot = args[0]
			}

			if err := os.MkdirAll(targetRoot, 0755); err != nil {
				return fmt.Errorf("failed to create project dir: %w", err)
			}

			curr := time.Now()

			fmt.Println(time.Since(curr))

			fmt.Println("✅ Scaffolding complete")

			return nil
		},
	}

	return cmd

}

const baseRawURL = "https://raw.githubusercontent.com/auth0-samples/auth0-acul-samples/monorepo-sample"

// GitHub API base for directory traversal
const baseTreeAPI = "https://api.github.com/repos/auth0-samples/auth0-acul-samples/git/trees/monorepo-sample?recursive=1"

// downloadRaw fetches a single file and saves it locally.
func downloadRaw(path, destDir string) error {
	url := fmt.Sprintf("%s/%s", baseRawURL, path)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch %s: %s", url, resp.Status)
	}

	// Create destination path
	destPath := filepath.Join(destDir, path)
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create dirs for %s: %w", destPath, err)
	}

	// Write file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", destPath, err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", destPath, err)
	}

	return nil
}

// GitHub tree API response
type treeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "blob" (file) or "tree" (dir)
	URL  string `json:"url"`
}

type treeResponse struct {
	Tree []treeEntry `json:"tree"`
}

// downloadDirectory downloads all files under a given directory using GitHub Tree API.
func downloadDirectory(dir, destDir string) error {
	resp, err := http.Get(baseTreeAPI)
	if err != nil {
		return fmt.Errorf("failed to fetch tree: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch tree API: %s", resp.Status)
	}

	var tr treeResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return fmt.Errorf("failed to decode tree: %w", err)
	}

	for _, entry := range tr.Tree {
		if entry.Type == "blob" && filepath.HasPrefix(entry.Path, dir) {
			if err := downloadRaw(entry.Path, destDir); err != nil {
				return err
			}
		}
	}
	return nil
}
