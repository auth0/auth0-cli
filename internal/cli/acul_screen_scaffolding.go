package cli

import (
	"encoding/json"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/auth0/auth0-cli/internal/utils"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"path/filepath"
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
			return runScaffoldAddScreen(cmd, args, destDir)
		},
	}

	destDirFlag.RegisterString(cmd, &destDir, ".")

	return cmd
}

func runScaffoldAddScreen(cmd *cobra.Command, args []string, destDir string) error {
	// Step 1: fetch manifest.json.
	manifest, err := LoadManifest()
	if err != nil {
		return err
	}

	// Step 2: read acul_config.json from destDir.
	aculConfig, err := LoadAculConfig(destDir)
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

	return nil

}

// Loads acul_config.json once
func LoadAculConfig(destDir string) (*AculConfig, error) {
	configPath := filepath.Join(destDir, "acul_config.json")
	data, err := ioutil.ReadFile(configPath)
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
