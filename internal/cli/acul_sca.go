package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/auth0/auth0-cli/internal/utils"
)

var templateFlag = Flag{
	Name:       "Template",
	LongForm:   "template",
	ShortForm:  "t",
	Help:       "Name of the template to use",
	IsRequired: false,
}

// This logic goes inside your `RunE` function.
func aculInitCmd2(_ *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init2",
		Args:  cobra.MaximumNArgs(1),
		Short: "Generate a new project from a template",
		Long:  `Generate a new project from a template.`,
		RunE:  runScaffold2,
	}

	return cmd
}

func runScaffold2(cmd *cobra.Command, args []string) error {
	// Step 1: fetch manifest.json.
	manifest, err := fetchManifest()
	if err != nil {
		return err
	}

	var chosenTemplate string
	if err := templateFlag.Select(cmd, &chosenTemplate, utils.FetchKeys(manifest.Templates), nil); err != nil {
		return handleInputError(err)
	}

	// Step 3: select screens.
	var screenOptions []string
	template := manifest.Templates[chosenTemplate]
	for _, s := range template.Screens {
		screenOptions = append(screenOptions, s.ID)
	}

	// Step 3: Let user select screens.
	var selectedScreens []string
	if err := prompt.AskMultiSelect("Select screens to include:", &selectedScreens, screenOptions...); err != nil {
		return err
	}

	// Step 3: Create project folder.
	var destDir string
	if len(args) < 1 {
		destDir = "my_acul_proj2"
	} else {
		destDir = args[0]
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create project dir: %w", err)
	}

	curr := time.Now()

	// --- Step 1: Download and Unzip to Temp Dir ---.
	repoURL := "https://github.com/auth0-samples/auth0-acul-samples/archive/refs/heads/monorepo-sample.zip"
	tempZipFile := downloadFile(repoURL)
	defer os.Remove(tempZipFile) // Clean up the temp zip file.

	tempUnzipDir, err := os.MkdirTemp("", "unzipped-repo-*")
	check(err, "Error creating temporary unzipped directory")
	defer os.RemoveAll(tempUnzipDir) // Clean up the entire temp directory.

	err = utils.Unzip(tempZipFile, tempUnzipDir)
	if err != nil {
		return err
	}

	// TODO: Adjust this prefix based on the actual structure of the unzipped content(once main branch is used).
	var sourcePathPrefix = "auth0-acul-samples-monorepo-sample/" + chosenTemplate

	// --- Step 2: Copy the Specified Base Directories ---.
	for _, dir := range manifest.Templates[chosenTemplate].BaseDirectories {
		// TODO: Remove hardcoding of removing the template - instead ensure to remove the template name in sourcePathPrefix.
		relPath, err := filepath.Rel(chosenTemplate, dir)
		if err != nil {
			continue
		}

		srcPath := filepath.Join(tempUnzipDir, sourcePathPrefix, relPath)
		destPath := filepath.Join(destDir, relPath)

		if _, err = os.Stat(srcPath); os.IsNotExist(err) {
			log.Printf("Warning: Source directory does not exist: %s", srcPath)
			continue
		}

		err = copyDir(srcPath, destPath)
		check(err, fmt.Sprintf("Error copying directory %s", dir))
	}

	// --- Step 3: Copy the Specified Base Files ---.
	for _, baseFile := range manifest.Templates[chosenTemplate].BaseFiles {
		// TODO: Remove hardcoding of removing the template - instead ensure to remove the template name in sourcePathPrefix.
		relPath, err := filepath.Rel(chosenTemplate, baseFile)
		if err != nil {
			continue
		}

		srcPath := filepath.Join(tempUnzipDir, sourcePathPrefix, relPath)
		destPath := filepath.Join(destDir, relPath)

		if _, err = os.Stat(srcPath); os.IsNotExist(err) {
			log.Printf("Warning: Source file does not exist: %s", srcPath)
			continue
		}

		parentDir := filepath.Dir(destPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			log.Printf("Error creating parent directory for %s: %v", baseFile, err)
			continue
		}

		err = copyFile(srcPath, destPath)
		check(err, fmt.Sprintf("Error copying file %s", baseFile))
	}

	screenInfo := createScreenMap(template.Screens)
	for _, s := range selectedScreens {
		screen := screenInfo[s]

		relPath, err := filepath.Rel(chosenTemplate, screen.Path)
		if err != nil {
			continue
		}

		srcPath := filepath.Join(tempUnzipDir, sourcePathPrefix, relPath)
		destPath := filepath.Join(destDir, relPath)

		if _, err = os.Stat(srcPath); os.IsNotExist(err) {
			log.Printf("Warning: Source directory does not exist: %s", srcPath)
			continue
		}

		parentDir := filepath.Dir(destPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			log.Printf("Error creating parent directory for %s: %v", screen.Path, err)
			continue
		}

		fmt.Printf("Copying screen path: %s\n", screen.Path)
		err = copyDir(srcPath, destPath)
		check(err, fmt.Sprintf("Error copying screen file %s", screen.Path))
	}

	fmt.Println(time.Since(curr))

	config := AculConfig{
		ChosenTemplate:      chosenTemplate,
		Screen:              selectedScreens,                 // If needed
		InitTimestamp:       time.Now().Format(time.RFC3339), // Standard time format
		AculManifestVersion: manifest.Metadata.Version,
	}

	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		panic(err) // or handle gracefully
	}

	// Build full path to acul_config.json inside destDir
	configPath := filepath.Join(destDir, "acul_config.json")

	err = os.WriteFile(configPath, b, 0644)
	if err != nil {
		fmt.Printf("Failed to write config: %v\n", err)
	}

	fmt.Println("\nProject successfully created!\n" +
		"Explore the sample app: https://github.com/auth0/acul-sample-app")

	return nil
}

// Helper function to handle errors and log them.
func check(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %v", err, msg)
	}
}

// Function to download a file from a URL to a temporary location.
func downloadFile(url string) string {
	tempFile, err := os.CreateTemp("", "github-zip-*.zip")
	check(err, "Error creating temporary file")

	fmt.Printf("Downloading from %s...\n", url)
	resp, err := http.Get(url)
	check(err, "Error downloading file")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Bad status code: %s", resp.Status)
	}

	_, err = io.Copy(tempFile, resp.Body)
	check(err, "Error saving zip file")
	tempFile.Close()

	return tempFile.Name()
}

// Function to copy a file from a source path to a destination path.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}
	return out.Close()
}

// Function to recursively copy a directory.
func copyDir(src, dst string) error {
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	err = os.MkdirAll(dst, sourceInfo.Mode())
	if err != nil {
		return err
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == src {
			return nil
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}
		return copyFile(path, destPath)
	})
}

func createScreenMap(screens []Screen) map[string]Screen {
	screenMap := make(map[string]Screen)
	for _, screen := range screens {
		screenMap[screen.ID] = screen
	}

	return screenMap
}

type AculConfig struct {
	ChosenTemplate      string   `json:"chosen_template"`
	Screen              []string `json:"screens"`        // if you want to track this
	InitTimestamp       string   `json:"init_timestamp"` // ISO8601 for readability
	AculManifestVersion string   `json:"acul_manifest_version"`
}
