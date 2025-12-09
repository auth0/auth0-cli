package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/auth0/auth0-cli/internal/utils"
)

type Manifest struct {
	Templates map[string]Template `json:"templates"`
	Metadata  Metadata            `json:"metadata"`
}

type Template struct {
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Framework       string    `json:"framework"`
	SDK             string    `json:"sdk"`
	BaseFiles       []string  `json:"base_files"`
	BaseDirectories []string  `json:"base_directories"`
	Screens         []Screens `json:"screens"`
}

type Screens struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Path        string   `json:"path"`
	ExtraFiles  []string `json:"extra_files"`
}

type Metadata struct {
	Version     string `json:"version"`
	Repository  string `json:"repository"`
	LastUpdated string `json:"last_updated"`
	Description string `json:"description"`
}

// loadManifest downloads and parses the manifest.json for the latest release.
func loadManifest(tag string) (*Manifest, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("https://raw.githubusercontent.com/auth0-samples/auth0-acul-samples/%s/manifest.json", tag)

	resp, err := client.Get(url)
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

// getLatestReleaseTag fetches the latest tag from GitHub API.
func getLatestReleaseTag() (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	url := "https://api.github.com/repos/auth0-samples/auth0-acul-samples/tags"

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch tags: received status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var tags []struct {
		Name string `json:"name"`
	}

	if err := json.Unmarshal(body, &tags); err != nil {
		return "", fmt.Errorf("failed to parse tags response: %w", err)
	}

	if len(tags) == 0 {
		return "", fmt.Errorf("no tags found in repository")
	}

	return tags[0].Name, nil
}

var (
	templateFlag = Flag{
		Name:       "Template",
		LongForm:   "template",
		ShortForm:  "t",
		Help:       "Template framework to use for your ACUL project.",
		IsRequired: false,
	}

	screensFlag = Flag{
		Name:       "Screens",
		LongForm:   "screens",
		ShortForm:  "s",
		Help:       "Comma-separated list of screens to include in your ACUL project.",
		IsRequired: false,
	}
)

// / aculInitCmd returns the cobra.Command for project initialization.
func aculInitCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Template string
		Screens  []string
	}

	cmd := &cobra.Command{
		Use:   "init",
		Args:  cobra.MaximumNArgs(1),
		Short: "Generate a new ACUL project from a template",
		Long: `Generate a new Advanced Customizations for Universal Login (ACUL) project from a template.
This command creates a new project with your choice of framework and authentication screens (login, signup, mfa, etc.). 
The generated project includes all necessary configuration and boilerplate code to get started with ACUL customizations.`,
		Example: `  auth0 acul init <app_name>
  auth0 acul init acul-sample-app
  auth0 acul init acul-sample-app --template react --screens login,signup
  auth0 acul init acul-sample-app -t react -s login,mfa,signup`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureACULPrerequisites(cmd.Context(), cli.api); err != nil {
				return err
			}

			return runScaffold(cli, cmd, args, &inputs)
		},
	}

	templateFlag.RegisterString(cmd, &inputs.Template, "")
	screensFlag.RegisterStringSlice(cmd, &inputs.Screens, []string{})

	return cmd
}

func runScaffold(cli *cli, cmd *cobra.Command, args []string, inputs *struct {
	Template string
	Screens  []string
}) error {
	if err := checkNodeInstallation(); err != nil {
		return err
	}

	latestTag, err := getLatestReleaseTag()
	if err != nil {
		return fmt.Errorf("failed to get latest release tag: %w", err)
	}

	manifest, err := loadManifest(latestTag)
	if err != nil {
		return err
	}

	chosenTemplate, err := selectTemplate(cmd, manifest, inputs.Template)
	if err != nil {
		return err
	}

	var availableScreenIDs []string
	for _, s := range manifest.Templates[chosenTemplate].Screens {
		availableScreenIDs = append(availableScreenIDs, s.ID)
	}

	selectedScreens, err := validateAndSelectScreens(cli, availableScreenIDs, inputs.Screens, true)
	if err != nil {
		return err
	}

	destDir := getDestDir(args)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create project dir: %w", err)
	}

	tempUnzipDir, err := downloadAndUnzipSampleRepo()
	if err != nil {
		return err
	}

	defer os.RemoveAll(tempUnzipDir)

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

	err = writeAculConfig(destDir, chosenTemplate, selectedScreens, manifest.Metadata.Version, latestTag)
	if err != nil {
		fmt.Printf("Failed to write config: %v\n", err)
	}

	runNpmGenerateScreenLoader(cli, destDir)

	if prompt.Confirm("Would you like to proceed with installing the required dependencies using 'npm install'?") {
		fmt.Println(ansi.Cyan("Installing Dependencies... This may take a little while."))
		runNpmInstall(cli, destDir)
	}

	showPostScaffoldingOutput(cli, destDir, "Project successfully created")

	return nil
}

func selectTemplate(cmd *cobra.Command, manifest *Manifest, providedTemplate string) (string, error) {
	var templateNames []string
	nameToKey := make(map[string]string)

	for key, template := range manifest.Templates {
		templateNames = append(templateNames, template.Name)
		nameToKey[template.Name] = key
	}

	// If template provided via flag, validate it.
	if providedTemplate != "" {
		for key, template := range manifest.Templates {
			if template.Name == providedTemplate || key == providedTemplate {
				return key, nil
			}
		}
		return "", fmt.Errorf("invalid template '%s'. Available templates: %s",
			providedTemplate, strings.Join(templateNames, ", "))
	}

	var chosenTemplateName string
	err := templateFlag.Select(cmd, &chosenTemplateName, templateNames, nil)
	if err != nil {
		return "", handleInputError(err)
	}

	return nameToKey[chosenTemplateName], nil
}

// ValidateAndSelectScreens validates provided screens or prompts for selection.
func validateAndSelectScreens(cli *cli, screenIDs, providedScreens []string, multiSelect bool) ([]string, error) {
	if len(screenIDs) == 0 {
		return nil, fmt.Errorf("no available screens found")
	}

	var valid []string
	var invalid []string

	for _, s := range providedScreens {
		screen := strings.TrimSpace(s)
		if screen == "" {
			continue
		}
		if slices.Contains(screenIDs, screen) {
			valid = append(valid, screen)
		} else {
			invalid = append(invalid, screen)
		}
	}

	// If user provided screens.
	if len(providedScreens) > 0 {
		if len(invalid) > 0 {
			cli.renderer.Warnf("Unsupported screens: %s", ansi.Red(strings.Join(invalid, ", ")))
		}

		if len(valid) > 0 {
			// If single-select, only return the first match.
			if !multiSelect {
				return []string{valid[0]}, nil
			}
			return valid, nil
		}

		cli.renderer.Warnf("No valid screen(s) found. Please select from the available options.")
	}

	// No valid provided screens — fall back to interactive selection.
	if multiSelect {
		var selected []string
		if err := prompt.AskMultiSelect("Select screens:", &selected, screenIDs...); err != nil {
			return nil, err
		}
		if len(selected) == 0 {
			return nil, fmt.Errorf("at least one screen must be selected")
		}
		return selected, nil
	}

	// Single select.
	var selected string
	q := prompt.SelectInput("screen", "Select a screen:", "", screenIDs, screenIDs[0], true)
	if err := prompt.AskOne(q, &selected); err != nil {
		return nil, err
	}

	return []string{selected}, nil
}

func getDestDir(args []string) string {
	if len(args) < 1 {
		return "acul-sample-app"
	}

	return args[0]
}

func downloadAndUnzipSampleRepo() (string, error) {
	latestTag, err := getLatestReleaseTag()
	if err != nil {
		return "", fmt.Errorf("failed to get latest release tag: %w", err)
	}

	repoURL := fmt.Sprintf("https://github.com/auth0-samples/auth0-acul-samples/archive/refs/tags/%s.zip", latestTag)
	tempZipFile, err := downloadFile(repoURL)
	if err != nil {
		return "", fmt.Errorf("error downloading sample repo: %w", err)
	}

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

// This supports any version tag (v1.0.0, v2.0.0, etc.) without hardcoding.
func findExtractedRepoDir(tempUnzipDir string) (string, error) {
	entries, err := os.ReadDir(tempUnzipDir)
	if err != nil {
		return "", fmt.Errorf("failed to read temp directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "auth0-acul-samples-") {
			return entry.Name(), nil
		}
	}

	return "", fmt.Errorf("could not find extracted auth0-acul-samples directory")
}

func copyTemplateBaseDirs(cli *cli, baseDirs []string, chosenTemplate, tempUnzipDir, destDir string) error {
	extractedDir, err := findExtractedRepoDir(tempUnzipDir)
	if err != nil {
		return fmt.Errorf("failed to find extracted directory: %w", err)
	}

	sourcePathPrefix := filepath.Join(extractedDir, chosenTemplate)
	for _, dirPath := range baseDirs {
		srcPath := filepath.Join(tempUnzipDir, sourcePathPrefix, dirPath)
		destPath := filepath.Join(destDir, dirPath)

		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			cli.renderer.Warnf("⚠️ Source directory does not exist: %s", ansi.Faint(srcPath))
			continue
		}

		if err := copyDir(srcPath, destPath); err != nil {
			return fmt.Errorf("error copying directory %s: %w", dirPath, err)
		}
	}

	return nil
}

func copyProjectTemplateFiles(cli *cli, baseFiles []string, chosenTemplate, tempUnzipDir, destDir string) error {
	extractedDir, err := findExtractedRepoDir(tempUnzipDir)
	if err != nil {
		return fmt.Errorf("failed to find extracted directory: %w", err)
	}

	sourcePathPrefix := filepath.Join(extractedDir, chosenTemplate)

	for _, filePath := range baseFiles {
		srcPath := filepath.Join(tempUnzipDir, sourcePathPrefix, filePath)
		destPath := filepath.Join(destDir, filePath)

		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			cli.renderer.Warnf("⚠️ Source file does not exist: %s", ansi.Faint(srcPath))
			continue
		}

		parentDir := filepath.Dir(destPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			cli.renderer.Warnf("❌ Error creating parent directory for %s: %v", ansi.Bold(filePath), err)
			continue
		}

		if err := copyFile(srcPath, destPath); err != nil {
			return fmt.Errorf("error copying file %s: %w", filePath, err)
		}
	}

	return nil
}

func copyProjectScreens(cli *cli, screens []Screens, selectedScreens []string, chosenTemplate, tempUnzipDir, destDir string) error {
	extractedDir, err := findExtractedRepoDir(tempUnzipDir)
	if err != nil {
		return fmt.Errorf("failed to find extracted directory: %w", err)
	}

	sourcePathPrefix := filepath.Join(extractedDir, chosenTemplate)
	screenInfo := createScreenMap(screens)
	for _, s := range selectedScreens {
		screen := screenInfo[s]

		srcPath := filepath.Join(tempUnzipDir, sourcePathPrefix, screen.Path)
		destPath := filepath.Join(destDir, screen.Path)

		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			cli.renderer.Warnf("%s Source directory does not exist: %s",
				ansi.Bold(ansi.Yellow("⚠️")), ansi.Faint(srcPath))
			continue
		}

		parentDir := filepath.Dir(destPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			cli.renderer.Warnf("❌ Error creating parent directory for %s: %v", ansi.Bold(screen.Path), err)
			continue
		}

		if err := copyDir(srcPath, destPath); err != nil {
			return fmt.Errorf("error copying screen directory %s: %w", screen.Path, err)
		}

		err = copyProjectTemplateFiles(cli, screen.ExtraFiles, chosenTemplate, tempUnzipDir, destDir)
		if err != nil {
			return err
		}
	}

	return nil
}

func writeAculConfig(destDir, chosenTemplate string, selectedScreens []string, manifestVersion, appVersion string) error {
	config := AculConfig{
		ChosenTemplate:      chosenTemplate,
		Screens:             selectedScreens,
		CreatedAt:           time.Now().UTC().Format(time.RFC3339),
		ModifiedAt:          time.Now().UTC().Format(time.RFC3339),
		AculManifestVersion: manifestVersion,
		AppVersion:          appVersion,
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

// downloadFile downloads a file from a URL to a temporary file and returns its name.
func downloadFile(url string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d when downloading %s", resp.StatusCode, url)
	}

	tempFile, err := os.CreateTemp("", "github-zip-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return "", fmt.Errorf("failed to save zip file: %w", err)
	}

	return tempFile.Name(), nil
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

func createScreenMap(screens []Screens) map[string]Screens {
	screenMap := make(map[string]Screens)
	for _, screen := range screens {
		screenMap[screen.ID] = screen
	}
	return screenMap
}

// showPostScaffoldingOutput displays comprehensive post-scaffolding information including
// success message, documentation, Node version check, next steps, and available commands.
func showPostScaffoldingOutput(cli *cli, destDir, successMessage string) {
	cli.renderer.Output("")
	cli.renderer.Infof("🎉 %s in %s!", successMessage, ansi.Bold(ansi.Cyan(fmt.Sprintf("'%s'", destDir))))
	cli.renderer.Output("")

	cli.renderer.Infof("📖  Explore the sample app: %s",
		ansi.Blue("https://github.com/auth0-samples/auth0-acul-samples"))
	cli.renderer.Output("")

	// Show next steps and related commands.
	fmt.Println()
	fmt.Println(ansi.Bold("Next Steps:"))
	fmt.Printf("  Navigate to %s\n", ansi.Cyan(destDir))
	fmt.Printf("  Run %s if dependencies are not installed\n", ansi.Cyan("npm install"))
	fmt.Printf("  Start the local dev server using %s\n", ansi.Cyan("auth0 acul dev"))

	printAvailableCommands()
	checkNodeVersion(cli)
}

func printAvailableCommands() {
	fmt.Println()
	fmt.Println(ansi.Bold("📋  Available Commands:"))
	fmt.Println()

	fmt.Printf("  %s - Add authentication screens\n", ansi.Bold(ansi.Green("auth0 acul screen add <screen-name>")))
	fmt.Printf("  %s - Local development with hot-reload\n", ansi.Bold(ansi.Green("auth0 acul dev")))
	fmt.Printf("  %s - Live sync changes to Auth0 tenant\n", ansi.Bold(ansi.Green("auth0 acul dev --connected")))
	fmt.Printf("  %s - Create starter config template\n", ansi.Bold(ansi.Green("auth0 acul config generate <screen>")))
	fmt.Printf("  %s - Pull current Auth0 settings\n", ansi.Bold(ansi.Green("auth0 acul config get <screen>")))
	fmt.Printf("  %s - Push local config to Auth0\n", ansi.Bold(ansi.Green("auth0 acul config set <screen>")))
	fmt.Printf("  %s - List all configurable screens\n", ansi.Bold(ansi.Green("auth0 acul config list")))

	fmt.Println()
	fmt.Printf("%s  Use %s to see all available commands\n",
		ansi.Yellow("💡 Tip:"), ansi.Cyan("auth0 acul --help"))
	fmt.Println()
}

type AculConfig struct {
	ChosenTemplate      string   `json:"chosen_template"`
	Screens             []string `json:"screens"`
	CreatedAt           string   `json:"created_at"`
	ModifiedAt          string   `json:"modified_at"`
	AppVersion          string   `json:"app_version"`
	AculManifestVersion string   `json:"acul_manifest_version"`
}

// checkNodeInstallation ensures that Node is installed and accessible in the system PATH.
func checkNodeInstallation() error {
	cmd := exec.Command("node", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("node is required but not found. Please install Node v22 or higher and try again")
	}
	return nil
}

// checkNodeVersion checks the major version number of the installed Node.
func checkNodeVersion(cli *cli) {
	cmd := exec.Command("node", "--version")
	output, err := cmd.Output()
	if err != nil {
		cli.renderer.Warnf(ansi.Yellow("Unable to detect Node version. Please ensure Node v22+ is installed."))
		return
	}

	version := strings.TrimSpace(string(output))
	re := regexp.MustCompile(`v?(\d+)\.`)
	matches := re.FindStringSubmatch(version)
	if len(matches) < 2 {
		cli.renderer.Warnf(ansi.Yellow(fmt.Sprintf("Unable to parse Node version: %s. Please ensure Node v22+ is installed.", version)))
		return
	}

	if major, _ := strconv.Atoi(matches[1]); major < 22 {
		fmt.Println(
			ansi.Yellow(fmt.Sprintf(
				"⚠️  Node %s detected. This project requires Node v22 or higher.\n"+
					"   Please upgrade to Node v22+ to run the sample app and build assets successfully.\n",
				version,
			)),
		)

		cli.renderer.Output("")
	}
}

// runNpmGenerateScreenLoader runs `npm run generate:screenLoader` in the given directory.
// Prints errors or warnings directly; silent if successful with no issues.
func runNpmGenerateScreenLoader(cli *cli, destDir string) {
	cmd := exec.Command("npm", "run", "generate:screenLoader")
	cmd.Dir = destDir

	output, err := cmd.CombinedOutput()
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	summary := strings.Join(lines, "\n")
	if len(lines) > 5 {
		summary = strings.Join(lines[:5], "\n") + "\n..."
	}

	if err != nil {
		cli.renderer.Warnf(
			"⚠️  Screen loader generation failed: %v\n"+
				"👉 Run manually: %s\n"+
				"📄 Required for: %s\n"+
				"💡 Tip: If it continues to fail, verify your Node setup and screen structure.",
			err,
			ansi.Bold(ansi.Cyan(fmt.Sprintf("cd %s && npm run generate:screenLoader", destDir))),
			ansi.Faint(fmt.Sprintf("%s/src/utils/screen/screenLoader.ts", destDir)),
		)

		if len(summary) > 0 {
			fmt.Println(summary)
		}

		return
	}
}

// runNpmInstall runs `npm install` in the given directory.
// Prints concise logs; warns on failure, silent if successful.
func runNpmInstall(cli *cli, destDir string) {
	cmd := exec.Command("npm", "install")
	cmd.Dir = destDir

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		cli.renderer.Warnf(
			"⚠️  npm install failed: %v\n"+
				"👉 Run manually: %s\n"+
				"📦 Directory: %s\n"+
				"💡 Tip: Check your Node.js and npm setup, or clear node_modules and retry.",
			err,
			ansi.Bold(ansi.Cyan(fmt.Sprintf("cd %s && npm install", destDir))),
			ansi.Faint(destDir),
		)
	}

	fmt.Println("✅ " + ansi.Green("All dependencies installed successfully"))
}
