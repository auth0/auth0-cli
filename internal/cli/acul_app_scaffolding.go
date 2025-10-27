package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// loadManifest loads manifest.json once.
func loadManifest() (*Manifest, error) {
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
  auth0 acul init my_acul_app
  auth0 acul init my_acul_app --template react --screens login,signup
  auth0 acul init my_acul_app -t react -s login,mfa,signup`,
		RunE: func(cmd *cobra.Command, args []string) error {
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

	manifest, err := loadManifest()
	if err != nil {
		return err
	}

	chosenTemplate, err := selectTemplate(cmd, manifest, inputs.Template)
	if err != nil {
		return err
	}

	selectedScreens, err := selectScreens(cli, manifest.Templates[chosenTemplate].Screens, inputs.Screens)
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

	runNpmGenerateScreenLoader(cli, destDir)

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

func selectScreens(cli *cli, screens []Screens, providedScreens []string) ([]string, error) {
	return validateAndSelectScreens(cli, screens, providedScreens)
}

// validateAndSelectScreens is a common function for screen validation and selection.
func validateAndSelectScreens(cli *cli, screens []Screens, providedScreens []string) ([]string, error) {
	var availableScreenIDs []string
	for _, s := range screens {
		availableScreenIDs = append(availableScreenIDs, s.ID)
	}

	if len(providedScreens) > 0 {
		var validScreens []string
		var invalidScreens []string

		for _, providedScreen := range providedScreens {
			if strings.TrimSpace(providedScreen) == "" {
				continue
			}

			found := false
			for _, availableScreen := range availableScreenIDs {
				if providedScreen == availableScreen {
					validScreens = append(validScreens, providedScreen)
					found = true
					break
				}
			}
			if !found {
				invalidScreens = append(invalidScreens, providedScreen)
			}
		}

		if len(invalidScreens) > 0 {
			cli.renderer.Warnf("%s The following screens are not supported for the chosen template: %s",
				ansi.Bold(ansi.Yellow("⚠️")),
				ansi.Bold(ansi.Red(strings.Join(invalidScreens, ", "))))
			cli.renderer.Infof("Available screens: %s",
				ansi.Bold(ansi.Cyan(strings.Join(availableScreenIDs, ", "))))
			cli.renderer.Infof("%s We're planning to support all screens in the future.",
				ansi.Faint("Note:"))
		}

		if len(validScreens) == 0 {
			cli.renderer.Warnf("%s None of the provided screens are valid for this template.",
				ansi.Bold(ansi.Yellow("⚠️")))
			cli.renderer.Infof("%s Please select from the available screens below:",
				ansi.Bold(ansi.Blue("→")))
			// Fall through to multi-select prompt.
		} else {
			return validScreens, nil
		}
	}

	// If no screens provided or no valid screens, prompt for multi-select.
	var selectedScreens []string
	err := prompt.AskMultiSelect("Select screens to include:", &selectedScreens, availableScreenIDs...)

	if len(selectedScreens) == 0 {
		return nil, fmt.Errorf("at least one screen must be selected")
	}

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
	sourcePathPrefix := filepath.Join("auth0-acul-samples-monorepo-sample", chosenTemplate)
	for _, dirPath := range baseDirs {
		srcPath := filepath.Join(tempUnzipDir, sourcePathPrefix, dirPath)
		destPath := filepath.Join(destDir, dirPath)

		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			cli.renderer.Warnf("%s Source directory does not exist: %s",
				ansi.Bold(ansi.Yellow("⚠️")), ansi.Faint(srcPath))
			continue
		}

		if err := copyDir(srcPath, destPath); err != nil {
			return fmt.Errorf("error copying directory %s: %w", dirPath, err)
		}
	}

	return nil
}

func copyProjectTemplateFiles(cli *cli, baseFiles []string, chosenTemplate, tempUnzipDir, destDir string) error {
	sourcePathPrefix := filepath.Join("auth0-acul-samples-monorepo-sample", chosenTemplate)

	for _, filePath := range baseFiles {
		srcPath := filepath.Join(tempUnzipDir, sourcePathPrefix, filePath)
		destPath := filepath.Join(destDir, filePath)

		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			cli.renderer.Warnf("%s Source file does not exist: %s",
				ansi.Bold(ansi.Yellow("⚠️")), ansi.Faint(srcPath))
			continue
		}

		parentDir := filepath.Dir(destPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			cli.renderer.Warnf("%s Error creating parent directory for %s: %v",
				ansi.Bold(ansi.Red("❌")), ansi.Bold(filePath), err)
			continue
		}

		if err := copyFile(srcPath, destPath); err != nil {
			return fmt.Errorf("error copying file %s: %w", filePath, err)
		}
	}

	return nil
}

func copyProjectScreens(cli *cli, screens []Screens, selectedScreens []string, chosenTemplate, tempUnzipDir, destDir string) error {
	sourcePathPrefix := "auth0-acul-samples-monorepo-sample/" + chosenTemplate
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
			cli.renderer.Warnf("%s Error creating parent directory for %s: %v",
				ansi.Bold(ansi.Red("❌")), ansi.Bold(screen.Path), err)
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
	cli.renderer.Infof("%s  %s in %s!",
		ansi.Bold(ansi.Green("🎉")), successMessage, ansi.Bold(ansi.Cyan(fmt.Sprintf("'%s'", destDir))))
	cli.renderer.Output("")

	cli.renderer.Infof("📖  Explore the sample app: %s",
		ansi.Blue("https://github.com/auth0-samples/auth0-acul-samples"))
	cli.renderer.Output("")

	checkNodeVersion(cli)

	// Show next steps and related commands.
	cli.renderer.Infof("%s Next Steps: Navigate to %s and run:", ansi.Bold("🚀"), ansi.Bold(ansi.Cyan(destDir)))
	cli.renderer.Infof("   1. %s", ansi.Bold(ansi.Cyan("npm install")))
	cli.renderer.Infof("   2. %s", ansi.Bold(ansi.Cyan("npm run build")))
	cli.renderer.Infof("   3. %s", ansi.Bold(ansi.Cyan("npm run screen dev")))
	cli.renderer.Output("")

	fmt.Printf("%s Available Commands:\n", ansi.Bold("📋"))
	fmt.Printf("   %s - Add more screens to your project\n",
		ansi.Bold(ansi.Green("auth0 acul screen add <screen-name>")))
	fmt.Printf("   %s - Generate a stub config file\n",
		ansi.Bold(ansi.Green("auth0 acul config generate <screen>")))
	fmt.Printf("   %s - Download current settings\n",
		ansi.Bold(ansi.Green("auth0 acul config get <screen>")))
	fmt.Printf("   %s - Upload customizations\n",
		ansi.Bold(ansi.Green("auth0 acul config set <screen>")))
	fmt.Printf("   %s - View available screens\n",
		ansi.Bold(ansi.Green("auth0 acul config list")))
	fmt.Println()

	fmt.Printf("%s %s: Use %s to see all available commands\n",
		ansi.Bold("💡"), ansi.Bold("Tip"), ansi.Bold(ansi.Cyan("'auth0 acul --help'")))
}

type AculConfig struct {
	ChosenTemplate      string   `json:"chosen_template"`
	Screens             []string `json:"screens"`
	InitTimestamp       string   `json:"init_timestamp"`
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
		cli.renderer.Output("")
		cli.renderer.Warnf(ansi.Yellow(fmt.Sprintf(" Node %s detected. This project requires Node %s or higher.",
			version, "v22")))
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
