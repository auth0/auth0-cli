package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var destDirFlag = Flag{
	Name:       "Destination Directory",
	LongForm:   "dir",
	ShortForm:  "d",
	Help:       "Path to existing project directory (must contain `acul_config.json`)",
	IsRequired: false,
}

func aculScreenAddCmd(cli *cli) *cobra.Command {
	var destDir string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add screens to an existing project",
		Long:  "Add screens to an existing project. The project must have been initialized using `auth0 acul init`.",
		Example: `  auth0 acul screen add <screen-name> <screen-name>... --dir <app-directory>
  auth0 acul screen add login-id login-password -d acul_app`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureACULPrerequisites(cmd.Context(), cli.api); err != nil {
				return err
			}

			pwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %v", err)
			}

			if len(destDir) < 1 {
				err = destDirFlag.Ask(cmd, &destDir, &pwd)
				if err != nil {
					return err
				}
			}

			return scaffoldAddScreen(cli, args, destDir)
		},
	}

	destDirFlag.RegisterString(cmd, &destDir, "")

	return cmd
}

// checkVersionCompatibility compares the user's ACUL config version with the latest available tag
// and warns if the project version is missing or outdated.
func checkVersionCompatibility(cli *cli, aculConfig *AculConfig, latestTag string) {
	if aculConfig.AppVersion == "" {
		cli.renderer.Warnf(
			ansi.Yellow("⚠️  Missing app version in acul_config.json. Reinitialize your project with `auth0 acul init`."),
		)
		return
	}

	if aculConfig.AppVersion != latestTag {
		compareLink := fmt.Sprintf(
			"https://github.com/auth0-samples/auth0-acul-samples/compare/%s...%s",
			aculConfig.AppVersion, latestTag,
		)

		cli.renderer.Warnf(
			ansi.Yellow(fmt.Sprintf("⚠️  ACUL project version outdated (%s). Check updates: %s",
				aculConfig.AppVersion, compareLink)),
		)
	}
}

func scaffoldAddScreen(cli *cli, args []string, destDir string) error {
	aculConfig, err := loadAculConfig(filepath.Join(destDir, "acul_config.json"))

	if err != nil {
		if os.IsNotExist(err) {
			cli.renderer.Warnf("couldn't find acul_config.json in destination directory. Please ensure you're in the right directory or have initialized the project using `auth0 acul init`")
			return nil
		}

		return err
	}

	latestTag, err := getLatestReleaseTag()
	if err != nil {
		return fmt.Errorf("failed to get latest release tag: %w", err)
	}

	manifest, err := loadManifest(aculConfig.AppVersion)
	if err != nil {
		return err
	}

	checkVersionCompatibility(cli, aculConfig, latestTag)

	selectedScreens, err := selectAndFilterScreens(cli, args, manifest, aculConfig.ChosenTemplate, aculConfig.Screens)
	if err != nil {
		return err
	}

	if err = addScreensToProject(cli, destDir, aculConfig.ChosenTemplate, selectedScreens, manifest.Templates[aculConfig.ChosenTemplate]); err != nil {
		return err
	}

	runNpmGenerateScreenLoader(cli, destDir)

	if err = updateAculConfigFile(destDir, aculConfig, selectedScreens); err != nil {
		return err
	}

	showPostScaffoldingOutput(cli, destDir, "Screens added successfully")

	return nil
}

func selectAndFilterScreens(cli *cli, args []string, manifest *Manifest, chosenTemplate string, existingScreens []string) ([]string, error) {
	var availableScreenIDs []string
	for _, s := range manifest.Templates[chosenTemplate].Screens {
		availableScreenIDs = append(availableScreenIDs, s.ID)
	}

	selectedScreens, err := validateAndSelectScreens(cli, availableScreenIDs, args)
	if err != nil {
		return nil, err
	}

	var finalScreens []string
	for _, s := range selectedScreens {
		exists := false
		for _, existing := range existingScreens {
			if s == existing {
				exists = true
				break
			}
		}

		if exists {
			promptMsg := fmt.Sprintf("Screen '%s' already exists. Do you want to overwrite its directory? ", s)
			if !prompt.Confirm(promptMsg) {
				continue
			}
		}
		finalScreens = append(finalScreens, s)
	}

	if len(finalScreens) == 0 {
		return nil, fmt.Errorf("no valid screens selected after filtering existing screens")
	}

	return finalScreens, nil
}

func addScreensToProject(cli *cli, destDir, chosenTemplate string, selectedScreens []string, selectedTemplate Template) error {
	tempUnzipDir, err := downloadAndUnzipSampleRepo()
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempUnzipDir) // Clean up the entire temp directory.

	var sourcePrefix = "auth0-acul-samples-monorepo-sample/" + chosenTemplate
	var sourceRoot = filepath.Join(tempUnzipDir, sourcePrefix)
	var destRoot = destDir

	missingFiles, editedFiles, err := processFiles(cli, selectedTemplate.BaseFiles, sourceRoot, destRoot)
	if err != nil {
		log.Printf("Error processing base files: %v", err)
	}

	missingDirFiles, editedDirFiles, err := processDirectories(cli, selectedTemplate.BaseDirectories, sourceRoot, destRoot)
	if err != nil {
		log.Printf("Error processing base directories: %v", err)
	}

	editedFiles = append(editedFiles, editedDirFiles...)
	missingFiles = append(missingFiles, missingDirFiles...)

	// Filter out screenLoader.ts since it gets regenerated by runNpmGenerateScreenLoader.
	filteredEditedFiles := filterOutScreenLoader(editedFiles)

	err = handleEditedFiles(cli, filteredEditedFiles, sourceRoot, destRoot)
	if err != nil {
		return fmt.Errorf("error during backup/overwrite: %w", err)
	}

	err = handleMissingFiles(cli, missingFiles, tempUnzipDir, sourcePrefix, destDir)
	if err != nil {
		return fmt.Errorf("error copying missing files: %w", err)
	}

	return copyProjectScreens(cli, selectedTemplate.Screens, selectedScreens, chosenTemplate, tempUnzipDir, destDir)
}

func handleEditedFiles(cli *cli, edited []string, sourceRoot, destRoot string) error {
	if len(edited) < 1 {
		return nil
	}

	fmt.Println("Edited files/directories may be overwritten:")
	for _, p := range edited {
		fmt.Println("  ", p)
	}

	fmt.Println("⚠️ DISCLAIMER: Some required base files and directories have been edited.\n" +
		"Your added screen(s) may NOT work correctly without these updates.\n" +
		"Proceeding without overwriting could lead to inconsistent or unstable behavior.")

	if !prompt.Confirm("Proceed with overwrite and backup? : ") {
		cli.renderer.Warnf("User opted not to overwrite modified files.")
		return nil
	}

	err := backupAndOverwrite(cli, edited, sourceRoot, destRoot)
	if err != nil {
		cli.renderer.Warnf("Error during backup and overwrite: %v\n", err)
		return err
	}

	cli.renderer.Infof(ansi.Bold(ansi.Blue("Edited files backed up to back_up folder and overwritten.")))

	return nil
}

// Copy missing files from source to destination.
func handleMissingFiles(cli *cli, missing []string, tempUnzipDir, sourcePrefix, destDir string) error {
	for _, baseFile := range missing {
		srcPath := filepath.Join(tempUnzipDir, sourcePrefix, baseFile)
		destPath := filepath.Join(destDir, baseFile)
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			cli.renderer.Warnf("Warning: Source file does not exist: %s", srcPath)
			continue
		}

		parentDir := filepath.Dir(destPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			cli.renderer.Warnf("Error creating parent dir for %s: %v", baseFile, err)
			continue
		}

		if err := copyFile(srcPath, destPath); err != nil {
			return fmt.Errorf("error copying file %s: %w", baseFile, err)
		}
	}
	return nil
}

// backupAndOverwrite backs up edited files, then overwrites them with source files.
func backupAndOverwrite(cli *cli, edited []string, sourceRoot, destRoot string) error {
	backupRoot := filepath.Join(destRoot, "back_up")

	// Remove existing backup folder if it exists.
	if _, err := os.Stat(backupRoot); err == nil {
		if err := os.RemoveAll(backupRoot); err != nil {
			return fmt.Errorf("failed to clear existing backup folder: %w", err)
		}
	}

	// Create a fresh backup folder.
	if err := os.MkdirAll(backupRoot, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	for _, relPath := range edited {
		destFile := filepath.Join(destRoot, relPath)
		backupFile := filepath.Join(backupRoot, relPath)
		sourceFile := filepath.Join(sourceRoot, relPath)

		if err := os.MkdirAll(filepath.Dir(backupFile), 0755); err != nil {
			cli.renderer.Warnf("Failed to create backup directory for %s: %v", relPath, err)
			continue
		}

		if err := copyFile(destFile, backupFile); err != nil {
			cli.renderer.Warnf("Failed to backup file %s: %v", relPath, err)
			continue
		}

		if err := copyFile(sourceFile, destFile); err != nil {
			cli.renderer.Errorf("Failed to overwrite file %s: %v", relPath, err)
			continue
		}

		cli.renderer.Infof("Overwritten: %s", relPath)
	}
	return nil
}

// processDirectories processes files in all base directories relative to chosenTemplate.
func processDirectories(cli *cli, baseDirs []string, sourceRoot, destRoot string) (missing, edited []string, err error) {
	for _, dir := range baseDirs {
		sourceDir := filepath.Join(sourceRoot, dir)
		files, listErr := listFilesInDir(sourceDir)
		if listErr != nil {
			err = fmt.Errorf("failed to list files in %s: %w", sourceDir, listErr)
			return
		}

		for _, sourceFile := range files {
			relPath, relErr := filepath.Rel(sourceRoot, sourceFile)
			if relErr != nil {
				continue
			}

			destFile := filepath.Join(destRoot, relPath)
			editedFlag, compErr := isFileEdited(sourceFile, destFile)
			switch {
			case compErr != nil && os.IsNotExist(compErr):
				missing = append(missing, relPath)
			case compErr != nil:
				cli.renderer.Warnf("Warning: failed to determine if file has been edited: %v", compErr)
				continue
			case editedFlag:
				edited = append(edited, relPath)
			}
		}
	}
	return
}

func listFilesInDir(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

func processFiles(cli *cli, baseFiles []string, sourceRoot, destRoot string) (missing, edited []string, err error) {
	for _, baseFile := range baseFiles {
		sourcePath := filepath.Join(sourceRoot, baseFile)
		destPath := filepath.Join(destRoot, baseFile)

		editedFlag, err := isFileEdited(sourcePath, destPath)
		switch {
		case err != nil && os.IsNotExist(err):
			missing = append(missing, baseFile)
		case err != nil:
			cli.renderer.Warnf("Warning: failed to determine if file has been edited: %v", err)
			continue
		case editedFlag:
			edited = append(edited, baseFile)
		}
	}
	return
}

func isFileEdited(source, dest string) (bool, error) {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return false, err
	}

	destInfo, err := os.Stat(dest)
	if os.IsNotExist(err) {
		return true, nil
	}

	if err != nil {
		return false, err
	}

	if sourceInfo.Size() != destInfo.Size() {
		return true, nil
	}

	hashSource, err := fileHash(source)
	if err != nil {
		return false, err
	}
	hashDest, err := fileHash(dest)
	if err != nil {
		return false, err
	}

	return !bytes.Equal(hashSource, hashDest), nil
}

func fileHash(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	// Use buffered copy for performance.
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// LoadAculConfig loads acul_config.json from the specified directory.
func loadAculConfig(configPath string) (*AculConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config AculConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// addUniqueScreens ensures selectedScreens are added uniquely to cfg.Screens.
func addUniqueScreens(cfg *AculConfig, selected []string) {
	existingSet := make(map[string]bool, len(cfg.Screens))
	for _, s := range cfg.Screens {
		existingSet[s] = true
	}

	for _, s := range selected {
		if !existingSet[s] {
			cfg.Screens = append(cfg.Screens, s)
			existingSet[s] = true
		}
	}
}

// updateAculConfigFile merges new screens into acul_config.json and updates metadata.
func updateAculConfigFile(destDir string, cfg *AculConfig, selectedScreens []string) error {
	if cfg == nil {
		return fmt.Errorf("aculConfig cannot be nil")
	}

	addUniqueScreens(cfg, selectedScreens)
	cfg.ModifiedAt = time.Now().UTC().Format(time.RFC3339)

	configPath := filepath.Join(destDir, "acul_config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal acul_config.json: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write acul_config.json: %w", err)
	}

	return nil
}

// since it gets regenerated by runNPMGenerate command anyway.
func filterOutScreenLoader(editedFiles []string) []string {
	var filtered []string
	for _, file := range editedFiles {
		// Skip only the specific screenLoader.ts file that gets regenerated.
		normalizedPath := filepath.ToSlash(file)
		if normalizedPath == "src/utils/screen/screenLoader.ts" {
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered
}
