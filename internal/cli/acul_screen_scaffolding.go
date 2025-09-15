package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/auth0/auth0-cli/internal/ansi"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/prompt"
)

var destDirFlag = Flag{
	Name:       "Destination Directory",
	LongForm:   "dir",
	ShortForm:  "d",
	Help:       "Path to existing project directory (must contain `acul_config.json`)",
	IsRequired: false,
}

var (
	aculConfigOnce   sync.Once
	aculConfigLoaded AculConfig
)

func aculScreenAddCmd(cli *cli) *cobra.Command {
	var destDir string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add screens to an existing project",
		Long:  "Add screens to an existing project.",
		RunE: func(cmd *cobra.Command, args []string) error {
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

func scaffoldAddScreen(cli *cli, args []string, destDir string) error {
	manifest, err := LoadManifest()
	if err != nil {
		return err
	}

	aculConfig, err := LoadAculConfig(filepath.Join(destDir, "acul_config.json"))
	if err != nil {
		return err
	}

	selectedScreens, err := chooseScreens(args, manifest, aculConfig.ChosenTemplate)
	if err != nil {
		return err
	}

	selectedScreens = filterScreensForOverwrite(selectedScreens, aculConfig.Screens)

	if err = addScreensToProject(cli, destDir, aculConfig.ChosenTemplate, selectedScreens, manifest.Templates[aculConfig.ChosenTemplate]); err != nil {
		return err
	}

	// Update acul_config.json with new screens.
	aculConfig.Screens = append(aculConfig.Screens, selectedScreens...)
	configBytes, err := json.MarshalIndent(aculConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated acul_config.json: %w", err)
	}

	if err := os.WriteFile(filepath.Join(destDir, "acul_config.json"), configBytes, 0644); err != nil {
		return fmt.Errorf("failed to write updated acul_config.json: %w", err)
	}

	cli.renderer.Infof(ansi.Bold(ansi.Green("Screens added successfully")))

	return nil
}

// Filter out screens user does not want to overwrite.
func filterScreensForOverwrite(selectedScreens []string, existingScreens []string) []string {
	var finalScreens []string
	for _, s := range selectedScreens {
		if screenExists(existingScreens, s) {
			promptMsg := fmt.Sprintf("Screen '%s' already exists. Do you want to overwrite its directory? (y/N): ", s)
			if !prompt.Confirm(promptMsg) {
				continue
			}
		}
		finalScreens = append(finalScreens, s)
	}
	return finalScreens
}

// Helper to check if a screen exists in the slice.
func screenExists(screens []string, target string) bool {
	for _, screen := range screens {
		if screen == target {
			return true
		}
	}
	return false
}

// Select screens: from args or prompt.
func chooseScreens(args []string, manifest *Manifest, chosenTemplate string) ([]string, error) {
	if len(args) != 0 {
		return args, nil
	}

	selectedScreens, err := selectScreens(manifest.Templates[chosenTemplate])
	if err != nil {
		return nil, err
	}

	return selectedScreens, nil
}

func addScreensToProject(cli *cli, destDir, chosenTemplate string, selectedScreens []string, selectedTemplate Template) error {
	tempUnzipDir, err := downloadAndUnzipSampleRepo()
	defer os.RemoveAll(tempUnzipDir) // Clean up the entire temp directory.
	if err != nil {
		return err
	}

	// TODO: Adjust this prefix based on the actual structure of the unzipped content(once main branch is used).
	var sourcePrefix = "auth0-acul-samples-monorepo-sample/" + chosenTemplate
	var sourceRoot = filepath.Join(tempUnzipDir, sourcePrefix)
	var destRoot = destDir

	missingFiles, editedFiles, err := processFiles(cli, selectedTemplate.BaseFiles, sourceRoot, destRoot, chosenTemplate)
	if err != nil {
		log.Printf("Error processing base files: %v", err)
	}

	missingDirFiles, editedDirFiles, err := processDirectories(cli, selectedTemplate.BaseDirectories, sourceRoot, destRoot, chosenTemplate)
	if err != nil {
		log.Printf("Error processing base directories: %v", err)
	}

	editedFiles = append(editedFiles, editedDirFiles...)
	missingFiles = append(missingFiles, missingDirFiles...)

	err = handleEditedFiles(cli, editedFiles, sourceRoot, destRoot)
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

	if !prompt.Confirm("Proceed with overwrite and backup? (y/N): ") {
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
	if len(missing) > 0 {
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
func processDirectories(cli *cli, baseDirs []string, sourceRoot, destRoot, chosenTemplate string) (missing, edited []string, err error) {
	for _, dir := range baseDirs {
		// TODO: Remove chosenTemplate prefix from dir to get relative base directory.
		baseDir, relErr := filepath.Rel(chosenTemplate, dir)
		if relErr != nil {
			return
		}

		sourceDir := filepath.Join(sourceRoot, baseDir)
		files, listErr := listFilesInDir(sourceDir)
		if listErr != nil {
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

func processFiles(cli *cli, baseFiles []string, sourceRoot, destRoot, chosenTemplate string) (missing, edited []string, err error) {
	for _, baseFile := range baseFiles {
		// TODO: Remove hardcoding of removing the template - instead ensure to remove the template name in sourcePathPrefix.
		relPath, err := filepath.Rel(chosenTemplate, baseFile)
		if err != nil {
			continue
		}

		sourcePath := filepath.Join(sourceRoot, relPath)
		destPath := filepath.Join(destRoot, relPath)

		editedFlag, err := isFileEdited(sourcePath, destPath)
		switch {
		case err != nil && os.IsNotExist(err):
			missing = append(missing, relPath)
		case err != nil:
			cli.renderer.Warnf("Warning: failed to determine if file has been edited: %v", err)
			continue
		case editedFlag:
			edited = append(edited, relPath)
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
	if err != nil && os.IsNotExist(err) {
		return false, err
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
	return !equalByteSlices(hashSource, hashDest), nil
}

func equalByteSlices(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

// LoadAculConfig loads acul_config.json once.
func LoadAculConfig(configPath string) (*AculConfig, error) {
	var configErr error
	aculConfigOnce.Do(func() {
		b, err := os.ReadFile(configPath)
		if err != nil {
			configErr = err
			return
		}
		err = json.Unmarshal(b, &aculConfigLoaded)
		if err != nil {
			configErr = err
		}
	})
	if configErr != nil {
		return nil, configErr
	}
	return &aculConfigLoaded, nil
}
