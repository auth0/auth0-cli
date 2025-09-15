package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/auth0/auth0-cli/internal/utils"
)

var (
	manifestLoaded Manifest // type Manifest should match your manifest schema
	manifestOnce   sync.Once
)

// LoadManifest Loads manifest.json once
func LoadManifest() (*Manifest, error) {
	url := "https://raw.githubusercontent.com/auth0-samples/auth0-acul-samples/monorepo-sample/manifest.json"
	var manifestErr error
	manifestOnce.Do(func() {
		resp, err := http.Get(url)
		if err != nil {
			manifestErr = fmt.Errorf("cannot fetch manifest: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			manifestErr = fmt.Errorf("failed to fetch manifest: received status code %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			manifestErr = fmt.Errorf("cannot read manifest body: %w", err)
		}

		if err := json.Unmarshal(body, &manifestLoaded); err != nil {
			manifestErr = fmt.Errorf("invalid manifest format: %w", err)
		}
	})

	if manifestErr != nil {
		return nil, manifestErr
	}

	return &manifestLoaded, nil
}

var templateFlag = Flag{
	Name:       "Template",
	LongForm:   "template",
	ShortForm:  "t",
	Help:       "Name of the template to use",
	IsRequired: false,
}

// aculInitCmd returns the cobra.Command for project initialization.
func aculInitCmd(cli *cli) *cobra.Command {
	return &cobra.Command{
		Use:     "init",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Generate a new project from a template",
		Long:    "Generate a new project from a template.",
		Example: `  acul init acul_project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScaffold2(cli, cmd, args)
		},
	}
}

func runScaffold2(cli *cli, cmd *cobra.Command, args []string) error {
	manifest, err := LoadManifest()
	if err != nil {
		return err
	}

	chosenTemplate, err := selectTemplate(cmd, manifest)
	if err != nil {
		return err
	}

	selectedScreens, err := selectScreens(manifest.Templates[chosenTemplate])
	if err != nil {
		return err
	}

	destDir := getDestDir(args)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create project dir: %w", err)
	}

	tempUnzipDir, err := downloadAndUnzipSampleRepo()
	defer os.RemoveAll(tempUnzipDir) // Clean up the entire temp directory.
	if err != nil {
		return err
	}

	selectedTemplate := manifest.Templates[chosenTemplate]

	err = copyTemplateBaseDirs(cli, selectedTemplate.BaseDirectories, chosenTemplate, tempUnzipDir, destDir)
	if err != nil {
		return err
	}

	err = copyProjectTemplateFiles(cli, selectedTemplate.BaseFiles, chosenTemplate, tempUnzipDir, destDir)
	if err != nil {
		return err
	}

	err = copyProjectScreens(cli, selectedTemplate.Screens, selectedScreens, chosenTemplate, tempUnzipDir, destDir)
	if err != nil {
		return err
	}

	err = writeAculConfig(destDir, chosenTemplate, selectedScreens, manifest.Metadata.Version)
	if err != nil {
		fmt.Printf("Failed to write config: %v\n", err)
	}

	fmt.Println("\nProject successfully created!\n" +
		"Explore the sample app: https://github.com/auth0/acul-sample-app")
	return nil
}

func selectTemplate(cmd *cobra.Command, manifest *Manifest) (string, error) {
	var chosenTemplate string
	err := templateFlag.Select(cmd, &chosenTemplate, utils.FetchKeys(manifest.Templates), nil)
	if err != nil {
		return "", handleInputError(err)
	}
	return chosenTemplate, nil
}

func selectScreens(template Template) ([]string, error) {
	var screenOptions []string
	for _, s := range template.Screens {
		screenOptions = append(screenOptions, s.ID)
	}
	var selectedScreens []string
	err := prompt.AskMultiSelect("Select screens to include:", &selectedScreens, screenOptions...)
	return selectedScreens, err
}

func getDestDir(args []string) string {
	if len(args) < 1 {
		return "my_acul_proj"
	}
	return args[0]
}

func downloadAndUnzipSampleRepo() (string, error) {
	repoURL := "https://github.com/auth0-samples/auth0-acul-samples/archive/refs/heads/monorepo-sample.zip"
	tempZipFile := downloadFile(repoURL)
	defer os.Remove(tempZipFile) // Clean up the temp zip file.

	tempUnzipDir, err := os.MkdirTemp("", "unzipped-repo-*")
	if err != nil {
		return "", fmt.Errorf("error creating temporary unzip dir: %w", err)
	}

	if err = utils.Unzip(tempZipFile, tempUnzipDir); err != nil {
		return "", err
	}

	return tempUnzipDir, nil
}

func copyTemplateBaseDirs(cli *cli, baseDirs []string, chosenTemplate, tempUnzipDir, destDir string) error {
	sourcePathPrefix := "auth0-acul-samples-monorepo-sample/" + chosenTemplate
	for _, dir := range baseDirs {
		// TODO: Remove hardcoding of removing the template - instead ensure to remove the template name in sourcePathPrefix.
		relPath, err := filepath.Rel(chosenTemplate, dir)
		if err != nil {
			continue
		}

		srcPath := filepath.Join(tempUnzipDir, sourcePathPrefix, relPath)
		destPath := filepath.Join(destDir, relPath)

		if _, err = os.Stat(srcPath); os.IsNotExist(err) {
			cli.renderer.Warnf("Warning: Source directory does not exist: %s", srcPath)
			continue
		}

		if err := copyDir(srcPath, destPath); err != nil {
			return fmt.Errorf("error copying directory %s: %w", dir, err)
		}
	}

	return nil
}

func copyProjectTemplateFiles(cli *cli, baseFiles []string, chosenTemplate, tempUnzipDir, destDir string) error {
	sourcePathPrefix := "auth0-acul-samples-monorepo-sample/" + chosenTemplate
	for _, baseFile := range baseFiles {
		// TODO: Remove hardcoding of removing the template - instead ensure to remove the template name in sourcePathPrefix.
		relPath, err := filepath.Rel(chosenTemplate, baseFile)
		if err != nil {
			continue
		}

		srcPath := filepath.Join(tempUnzipDir, sourcePathPrefix, relPath)
		destPath := filepath.Join(destDir, relPath)

		if _, err = os.Stat(srcPath); os.IsNotExist(err) {
			cli.renderer.Warnf("Warning: Source file does not exist: %s", srcPath)
			continue
		}

		parentDir := filepath.Dir(destPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			cli.renderer.Warnf("Error creating parent directory for %s: %v", baseFile, err)
			continue
		}

		if err := copyFile(srcPath, destPath); err != nil {
			return fmt.Errorf("error copying file %s: %w", baseFile, err)
		}
	}

	return nil
}

func copyProjectScreens(cli *cli, screens []Screen, selectedScreens []string, chosenTemplate, tempUnzipDir, destDir string) error {
	sourcePathPrefix := "auth0-acul-samples-monorepo-sample/" + chosenTemplate
	screenInfo := createScreenMap(screens)
	for _, s := range selectedScreens {
		screen := screenInfo[s]

		relPath, err := filepath.Rel(chosenTemplate, screen.Path)
		if err != nil {
			continue
		}

		srcPath := filepath.Join(tempUnzipDir, sourcePathPrefix, relPath)
		destPath := filepath.Join(destDir, relPath)

		if _, err = os.Stat(srcPath); os.IsNotExist(err) {
			cli.renderer.Warnf("Warning: Source directory does not exist: %s", srcPath)
			continue
		}

		parentDir := filepath.Dir(destPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			cli.renderer.Warnf("Error creating parent directory for %s: %v", screen.Path, err)
			continue
		}

		if err := copyDir(srcPath, destPath); err != nil {
			return fmt.Errorf("error copying screen directory %s: %w", screen.Path, err)
		}
	}

	return nil
}

func writeAculConfig(destDir, chosenTemplate string, selectedScreens []string, manifestVersion string) error {
	config := AculConfig{
		ChosenTemplate:      chosenTemplate,
		Screens:             selectedScreens,
		InitTimestamp:       time.Now().Format(time.RFC3339),
		AculManifestVersion: manifestVersion,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(destDir, "acul_config.json")
	if err = os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	return nil
}

// Helper function to handle errors and log them, exiting the process.
func check(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %v", msg, err)
	}
}

// downloadFile downloads a file from a URL to a temporary file and returns its name.
func downloadFile(url string) string {
	tempFile, err := os.CreateTemp("", "github-zip-*.zip")
	check(err, "Error creating temporary file")

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

	if _, err = io.Copy(out, in); err != nil {
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
	Screens             []string `json:"screens"`
	InitTimestamp       string   `json:"init_timestamp"`
	AculManifestVersion string   `json:"acul_manifest_version"`
}
