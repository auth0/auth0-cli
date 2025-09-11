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

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/auth0/auth0-cli/internal/utils"
)

var destDirFlag = Flag{
	Name:       "Destination Directory",
	LongForm:   "dir",
	ShortForm:  "d",
	Help:       "Path to existing project directory (must contain `acul_config.json`)",
	IsRequired: false,
}

func aculAddScreenCmd(_ *cli) *cobra.Command {
	var destDir string
	cmd := &cobra.Command{
		Use:   "add-screen",
		Short: "Add screens to an existing project",
		Long:  `Add screens to an existing project.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get current working directory
			pwd, err := os.Getwd()
			if err != nil {
				log.Fatalf("Failed to get current directory: %v", err)
			}

			if len(destDir) < 1 {
				err = destDirFlag.Ask(cmd, &destDir, &pwd)
				if err != nil {
					return err
				}
			} else {
				destDir = args[0]
			}

			return runScaffoldAddScreen(cmd, args, destDir)
		},
	}

	destDirFlag.RegisterString(cmd, &destDir, "")

	return cmd
}

func runScaffoldAddScreen(cmd *cobra.Command, args []string, destDir string) error {
	// Step 1: fetch manifest.json.
	manifest, err := LoadManifest()
	if err != nil {
		return err
	}

	// Step 2: read acul_config.json from destDir.
	aculConfig, err := LoadAculConfig(filepath.Join(destDir, "acul_config.json"))
	if err != nil {
		return err
	}

	// Step 2: select screens.
	var selectedScreens []string

	if len(args) != 0 {
		selectedScreens = args
	} else {
		var screenOptions []string

		for _, s := range manifest.Templates[aculConfig.ChosenTemplate].Screens {
			screenOptions = append(screenOptions, s.ID)
		}

		if err = prompt.AskMultiSelect("Select screens to include:", &selectedScreens, screenOptions...); err != nil {
			return err
		}
	}

	// Step 3: Add screens to existing project.
	if err = addScreensToProject(destDir, aculConfig.ChosenTemplate, selectedScreens); err != nil {
		return err
	}

	return nil
}

func addScreensToProject(destDir, chosenTemplate string, selectedScreens []string) error {
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
	var sourceRoot = filepath.Join(tempUnzipDir, sourcePathPrefix)

	var destRoot = destDir

	missingFiles, _, editedFiles, err := processFiles(manifestLoaded.Templates[chosenTemplate].BaseFiles, sourceRoot, destRoot, chosenTemplate)
	if err != nil {
		log.Printf("Error processing base files: %v", err)
	}

	missingDirFiles, _, editedDirFiles, err := processDirectories(manifestLoaded.Templates[chosenTemplate].BaseDirectories, sourceRoot, destRoot, chosenTemplate)
	if err != nil {
		log.Printf("Error processing base directories: %v", err)
	}

	allEdited := append(editedFiles, editedDirFiles...)
	allMissing := append(missingFiles, missingDirFiles...)

	if len(allEdited) > 0 {
		fmt.Printf("The following files/directories have been edited and may be overwritten:\n")
		for _, p := range allEdited {
			fmt.Println("  ", p)
		}

		// Show disclaimer before asking for confirmation
		fmt.Println("⚠️ DISCLAIMER: Some required base files and directories have been edited.\n" +
			"Your added screen(s) may NOT work correctly without these updates.\n" +
			"Proceeding without overwriting could lead to inconsistent or unstable behavior.")

		// Now ask for confirmation
		if confirmed := prompt.Confirm("Proceed with overwrite and backup? (y/N): "); !confirmed {
			fmt.Println("Operation aborted. No files were changed.")
			// Handle abort scenario here (return, exit, etc.)
		} else {
			err = backupAndOverwrite(allEdited, sourceRoot, destRoot)
			if err != nil {
				fmt.Printf("Backup and overwrite operation finished with errors: %v\n", err)
			} else {
				fmt.Println("All edited files have been backed up and overwritten successfully.")
			}
		}
	}

	fmt.Println("all missing files:", allMissing)
	if len(allMissing) > 0 {
		for _, baseFile := range allMissing {
			// TODO: Remove hardcoding of removing the template - instead ensure to remove the template name in sourcePathPrefix.
			//relPath, err := filepath.Rel(chosenTemplate, baseFile)
			//if err != nil {
			//	continue
			//}

			srcPath := filepath.Join(tempUnzipDir, sourcePathPrefix, baseFile)
			destPath := filepath.Join(destDir, baseFile)

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
		// Copy missing files and directories
	}

	screenInfo := createScreenMap(manifestLoaded.Templates[chosenTemplate].Screens)
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

	return nil
}

// backupAndOverwrite backs up edited files, then overwrites them with source files
func backupAndOverwrite(allEdited []string, sourceRoot, destRoot string) error {
	backupRoot := filepath.Join(destRoot, "back_up")

	// Create back_up directory if it doesn't exist
	if err := os.MkdirAll(backupRoot, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	for _, relPath := range allEdited {
		destFile := filepath.Join(destRoot, relPath)
		backupFile := filepath.Join(backupRoot, relPath)
		sourceFile := filepath.Join(sourceRoot, relPath)

		// Backup only if file exists in destination
		// Ensure backup directory exists
		if err := os.MkdirAll(filepath.Dir(backupFile), 0755); err != nil {
			fmt.Printf("Warning: failed to create backup dir for %s: %v\n", relPath, err)
			continue
		}
		// copyFile overwrites backupFile if it exists
		if err := copyFile(destFile, backupFile); err != nil {
			fmt.Printf("Warning: failed to backup file %s: %v\n", relPath, err)
			continue
		}
		fmt.Printf("Backed up: %s\n", relPath)

		// Overwrite destination with source file
		if err := copyFile(sourceFile, destFile); err != nil {
			fmt.Printf("Error overwriting file %s: %v\n", relPath, err)
			continue
		}
		fmt.Printf("Overwritten: %s\n", relPath)
	}
	return nil
}

// processDirectories processes files in all base directories relative to chosenTemplate,
// returning slices of missing, identical, and edited relative file paths.
func processDirectories(baseDirs []string, sourceRoot, destRoot, chosenTemplate string) (missing, identical, edited []string, err error) {
	for _, dir := range baseDirs {
		// Remove chosenTemplate prefix from dir to get relative base directory
		baseDir, relErr := filepath.Rel(chosenTemplate, dir)
		if relErr != nil {
			return nil, nil, nil, relErr
		}

		sourceDir := filepath.Join(sourceRoot, baseDir)
		files, listErr := listFilesInDir(sourceDir)
		if listErr != nil {
			return nil, nil, nil, listErr
		}

		for _, sourceFile := range files {
			relPath, relErr := filepath.Rel(sourceRoot, sourceFile)
			if relErr != nil {
				return nil, nil, nil, relErr
			}

			destFile := filepath.Join(destRoot, relPath)
			editedFlag, compErr := isFileEdited(sourceFile, destFile)
			switch {
			case compErr != nil && os.IsNotExist(compErr):
				missing = append(missing, relPath)
			case compErr != nil:
				return nil, nil, nil, compErr
			case editedFlag:
				edited = append(edited, relPath)
			default:
				identical = append(identical, relPath)
			}
		}
	}
	return missing, identical, edited, nil
}

// Get all files in a directory recursively (for base_directories)
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

func processFiles(baseFiles []string, sourceRoot, destRoot, chosenTemplate string) (missing, identical, edited []string, err error) {
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
			fmt.Println("Warning: failed to determine if file has been edited:", err)
			continue
		case editedFlag:
			edited = append(edited, relPath)
		default:
			identical = append(identical, relPath)
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
	// Fallback to hash comparison
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

// Returns SHA256 hash of file at given path
func fileHash(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	// Use buffered copy for performance
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// LoadAculConfig Loads acul_config.json once
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
