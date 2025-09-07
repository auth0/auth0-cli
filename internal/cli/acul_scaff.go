package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/auth0/auth0-cli/internal/utils"
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
func aculInitCmd1(_ *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init1",
		Args:  cobra.MaximumNArgs(1),
		Short: "Generate a new project from a template",
		Long:  `Generate a new project from a template.`,
		RunE:  runScaffold,
	}

	return cmd
}

func runScaffold(cmd *cobra.Command, args []string) error {
	// Step 1: fetch manifest.json.
	manifest, err := fetchManifest()
	if err != nil {
		return err
	}

	// Step 2: select template.
	var chosen string
	promptText := prompt.SelectInput("", "Select a template", "Chosen template(Todo)", utils.FetchKeys(manifest.Templates), "react-js", true)
	if err = prompt.AskOne(promptText, &chosen); err != nil {
		return err
	}

	// Step 3: select screens.
	var screenOptions []string
	template := manifest.Templates[chosen]
	for _, s := range template.Screens {
		screenOptions = append(screenOptions, s.ID)
	}

	// Step 3: Let user select screens.
	var selectedScreens []string
	if err := prompt.AskMultiSelect("Select screens to include:", &selectedScreens, screenOptions...); err != nil {
		return err
	}

	// Step 3: Create project folder.
	var projectDir string
	if len(args) < 1 {
		projectDir = "my_acul_proj1"
	} else {
		projectDir = args[0]
	}
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project dir: %w", err)
	}

	curr := time.Now()

	// Step 4: Init git repo.
	repoURL := "https://github.com/auth0-samples/auth0-acul-samples.git"
	if err := runGit(projectDir, "init"); err != nil {
		return err
	}
	if err := runGit(projectDir, "remote", "add", "-f", "origin", repoURL); err != nil {
		return err
	}
	if err := runGit(projectDir, "config", "core.sparseCheckout", "true"); err != nil {
		return err
	}

	// Step 5: Write sparse-checkout paths.
	baseFiles := manifest.Templates[chosen].BaseFiles
	baseDirectories := manifest.Templates[chosen].BaseDirectories

	var paths []string
	paths = append(paths, baseFiles...)
	paths = append(paths, baseDirectories...)

	for _, scr := range template.Screens {
		for _, chosenScreen := range selectedScreens {
			if scr.ID == chosenScreen {
				paths = append(paths, scr.Path)
			}
		}
	}

	sparseFile := filepath.Join(projectDir, ".git", "info", "sparse-checkout")

	f, err := os.Create(sparseFile)
	if err != nil {
		return fmt.Errorf("failed to write sparse-checkout file: %w", err)
	}

	for _, p := range paths {
		_, _ = f.WriteString(p + "\n")
	}

	f.Close()

	// Step 6: Pull only sparse files.
	if err := runGit(projectDir, "pull", "origin", "monorepo-sample"); err != nil {
		return err
	}

	// Step 7: Clean up .git.
	if err := os.RemoveAll(filepath.Join(projectDir, ".git")); err != nil {
		return fmt.Errorf("failed to clean up git metadata: %w", err)
	}

	fmt.Println(time.Since(curr))

	fmt.Printf("✅ Project scaffolded successfully in %s\n", projectDir)
	return nil
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
