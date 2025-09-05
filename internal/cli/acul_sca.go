package cli

import (
	"fmt"
	"github.com/auth0/auth0-cli/internal/utils"
	"github.com/spf13/cobra"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/auth0/auth0-cli/internal/prompt"
)

// This logic goes inside your `RunE` function.
func aculInitCmd2(c *cli) *cobra.Command {
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
	// Step 1: fetch manifest.json
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
		fmt.Println(err)
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

	// Step 3: Create project folder
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

	// --- Step 1: Download and Unzip to Temp Dir ---
	repoURL := "https://github.com/auth0-samples/auth0-acul-samples/archive/refs/heads/monorepo-sample.zip"
	tempZipFile := downloadFile(repoURL)
	defer os.Remove(tempZipFile) // Clean up the temp zip file

	tempUnzipDir, err := os.MkdirTemp("", "unzipped-repo-*")
	check(err, "Error creating temporary unzipped directory")
	defer os.RemoveAll(tempUnzipDir) // Clean up the entire temp directory

	err = utils.Unzip(tempZipFile, tempUnzipDir)
	if err != nil {
		return err
	}

	// TODO: Adjust this prefix based on the actual structure of the unzipped content(once main branch is used)
	const sourcePathPrefix = "auth0-acul-samples-monorepo-sample/"

	// --- Step 2: Copy the Specified Base Directories ---
	for _, dir := range manifest.Templates[chosen].BaseDirectories {
		srcPath := filepath.Join(tempUnzipDir, sourcePathPrefix, dir)
		destPath := filepath.Join(destDir, dir)

		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			log.Printf("Warning: Source directory does not exist: %s", srcPath)
			continue
		}

		fmt.Printf("Copying directory: %s\n", dir)
		err := copyDir(srcPath, destPath)
		check(err, fmt.Sprintf("Error copying directory %s", dir))
	}

	// --- Step 3: Copy the Specified Base Files ---
	for _, baseFile := range manifest.Templates[chosen].BaseFiles {
		srcPath := filepath.Join(tempUnzipDir, sourcePathPrefix, baseFile)
		destPath := filepath.Join(destDir, baseFile)

		if _, err = os.Stat(srcPath); os.IsNotExist(err) {
			log.Printf("Warning: Source file does not exist: %s", srcPath)
			continue
		}

		//parentDir := filepath.Dir(destPath)
		//if err := os.MkdirAll(parentDir, 0755); err != nil {
		//	log.Printf("Error creating parent directory for %s: %v", baseFile, err)
		//	continue
		//}

		fmt.Printf("Copying file: %s\n", baseFile)
		err := copyFile(srcPath, destPath)
		check(err, fmt.Sprintf("Error copying file %s", baseFile))
	}

	screenInfo := createScreenMap(template.Screens)
	for _, s := range selectedScreens {
		screen := screenInfo[s]

		srcPath := filepath.Join(tempUnzipDir, sourcePathPrefix, screen.Path)
		destPath := filepath.Join(destDir, screen.Path)

		if _, err = os.Stat(srcPath); os.IsNotExist(err) {
			log.Printf("Warning: Source directory does not exist: %s", srcPath)
			continue
		}

		//parentDir := filepath.Dir(destPath)
		//if err := os.MkdirAll(parentDir, 0755); err != nil {
		//	log.Printf("Error creating parent directory for %s: %v", screen.Path, err)
		//	continue
		//}

		fmt.Printf("Copying screen file: %s\n", screen.Path)
		err := copyFile(srcPath, destPath)
		check(err, fmt.Sprintf("Error copying screen file %s", screen.Path))

	}

	fmt.Println("\nSuccess! The files and directories have been copied.")

	fmt.Println(time.Since(curr))

	return nil
}

// Helper function to handle errors and log them
func check(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %v", err, msg)
	}
}

// Function to download a file from a URL to a temporary location
func downloadFile(url string) string {
	tempFile, err := os.CreateTemp("", "github-zip-*.zip")
	check(err, "Error creating temporary file")

	fmt.Printf("Downloading from %s...\n", url)
	resp, err := http.Get(url)
	check(err, "Error downloading file")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Bad status code: %s", resp.Status)
	}

	_, err = io.Copy(tempFile, resp.Body)
	check(err, "Error saving zip file")
	tempFile.Close()

	return tempFile.Name()
}

// Function to copy a file from a source path to a destination path
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

// Function to recursively copy a directory
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
		screenMap[screen.Name] = screen
	}
	return screenMap
}
