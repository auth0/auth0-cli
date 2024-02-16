package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

const (
	partialsDocsURL = "https://auth0.com/docs/customize/universal-login-pages/customize-signup-and-login-prompts"
)

var (
	partialsSegmentTypes = []management.PartialsPromptSegment{
		management.PartialsPromptLogin,
		management.PartialsPromptLoginID,
		management.PartialsPromptLoginPassword,
		management.PartialsPromptSignup,
		management.PartialsPromptSignupID,
		management.PartialsPromptSignupPassword,
	}
	partialsPrompt = Argument{
		Name: "Prompt",
		Help: "The prompt segment you want to modify.",
	}
	partialsSegmentFields = map[string]string{
		"form-content-start":      "Form Content Start",
		"form-content-end":        "Form Content End",
		"form-footer-start":       "Form Footer Start",
		"form-footer-end":         "Form Footer End",
		"secondary-actions-start": "Secondary Actions Start",
		"secondary-actions-end":   "Secondary Actions End",
	}
)

type promptsPartialsInput struct {
	Segment        string
	InputFile      string
	PartialsPrompt management.PartialsPrompt
}

func promptsPartialsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "partials",
		Short: "Manage partials for prompts",
		Long:  fmt.Sprintf("Manage [partials for prompts](%s)", partialsDocsURL),
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	cmd.AddCommand(partialsShowCmd(cli))
	cmd.AddCommand(partialsCreateCmd(cli))
	cmd.AddCommand(partialsUpdateCmd(cli))
	cmd.AddCommand(partialsDeleteCmd(cli))

	return cmd
}

func partialsShowCmd(cli *cli) *cobra.Command {
	var inputs promptsPartialsInput

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show partials for a prompt segment",
		Long:  "Show partials for a prompt segment.",
		Example: `	auth0 universal-login partials show <prompt>
	auth0 ul partials show login`,
		RunE: showPromptsPartials(cli, &inputs),
	}

	return cmd
}

func partialsCreateCmd(cli *cli) *cobra.Command {
	var inputs promptsPartialsInput

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.MaximumNArgs(1),
		Short: "Create partials for a prompt segment",
		Long:  "Create partials for a prompt segment.",
		Example: `	auth0 universal-login partials create <prompt>
	auth0 ul partials create <prompt> --input-file <input-file>
	auth0 ul partials create login --input-file /tmp/login/input-file.json`,
		RunE: createPromptsPartials(cli, &inputs),
	}

	cmd.Flags().StringVar(&inputs.InputFile, "input-file", "", "Path to a file that contains partial definitions for a prompt segment.")
	addCommonPromptPartialsFlags(cmd, &inputs)

	return cmd
}

func partialsUpdateCmd(cli *cli) *cobra.Command {
	var inputs promptsPartialsInput

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update partials for a prompt segment",
		Long:  "Update partials for a prompt segment.",
		Example: `	auth0 universal-login partials update <prompt>
	auth0 ul partials update <prompt> --input-file <input-file>
	auth0 ul partials update login --input-file /tmp/login/input-file.json`,
		RunE: updatePromptsPartials(cli, &inputs),
	}

	cmd.Flags().StringVar(&inputs.InputFile, "input-file", "", "Path to a file that contains partial definitions for a prompt segment.")
	addCommonPromptPartialsFlags(cmd, &inputs)

	return cmd
}

func partialsDeleteCmd(cli *cli) *cobra.Command {
	var inputs promptsPartialsInput

	cmd := &cobra.Command{
		Use:     "delete",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Delete partials for a prompt segment",
		Long:    "Delete partials for a prompt segment.",
		Example: `	auth0 universal-login partials delete <prompt>`,
		RunE:    deletePromptsPartials(cli, &inputs),
	}

	return cmd
}

func showPromptsPartials(cli *cli, inputs *promptsPartialsInput) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := checkPromptsPartialsAvailable(cli); err != nil {
			return fmt.Errorf("error minimum requirements not met: %w", err)
		}

		if len(args) == 0 {
			if err := partialsPrompt.Pick(cmd, &inputs.Segment, promptsPartialsSegmentOptions); err != nil {
				return err
			}
		} else {
			if !validSegment(args[0]) {
				return errorInvalidSegment(args[0])
			}
			inputs.Segment = args[0]
		}

		partials := &inputs.PartialsPrompt
		if err := ansi.Waiting(func() (err error) {
			partials, err = cli.api.Prompt.ReadPartials(context.Background(), management.PartialsPromptSegment(inputs.Segment))
			return err
		}); err != nil {
			return fmt.Errorf("failed to fetch partials for prompt segment (%s)", ansi.Bold(inputs.Segment))
		}

		partialsJSON, err := json.MarshalIndent(partials, "", "    ")
		if err != nil {
			return fmt.Errorf("failed to serialize the partials for prompt segment (%s) to json: %w", ansi.Bold(inputs.Segment), err)
		}

		cli.renderer.PromptsPartialsShow(string(partialsJSON), inputs.Segment)

		return nil
	}
}

func createPromptsPartials(cli *cli, inputs *promptsPartialsInput) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := checkPromptsPartialsAvailable(cli); err != nil {
			return fmt.Errorf("error minimum requirements not met: %w", err)
		}

		if len(args) == 0 {
			if err := partialsPrompt.Pick(cmd, &inputs.Segment, promptsPartialsSegmentOptions); err != nil {
				return err
			}
		} else {
			if !validSegment(args[0]) {
				return errorInvalidSegment(args[0])
			}
			inputs.Segment = args[0]
		}

		inputs.PartialsPrompt.Segment = management.PartialsPromptSegment(inputs.Segment)

		if err := promptPartialsFromInputs(inputs); err != nil {
			return err
		}

		if ok, _ := cmd.Flags().GetBool("no-input"); !ok {
			if err := editPartialsPrompt(cli, cmd, inputs); err != nil {
				return fmt.Errorf("failed to edit partials for prompt segment (%s): %w", ansi.Bold(inputs.Segment), err)
			}
		}

		if err := ansi.Waiting(func() error {
			return cli.api.Prompt.CreatePartials(context.Background(), &inputs.PartialsPrompt)
		}); err != nil {
			return fmt.Errorf("failed to create partials for prompt segment (%s): %w", ansi.Bold(inputs.Segment), err)
		}

		partialsJSON, err := json.MarshalIndent(inputs.PartialsPrompt, "", "    ")
		if err != nil {
			return fmt.Errorf("failed to serialize the partials for prompt segment (%s) to json: %w", ansi.Bold(inputs.Segment), err)
		}

		cli.renderer.PromptsPartialsCreate(string(partialsJSON), inputs.Segment)

		return nil
	}
}

func updatePromptsPartials(cli *cli, inputs *promptsPartialsInput) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := checkPromptsPartialsAvailable(cli); err != nil {
			return fmt.Errorf("error minimum requirements not met: %w", err)
		}

		if len(args) == 0 {
			if err := partialsPrompt.Pick(cmd, &inputs.Segment, promptsPartialsSegmentOptions); err != nil {
				return err
			}
		} else {
			if !validSegment(args[0]) {
				return errorInvalidSegment(args[0])
			}
			inputs.Segment = args[0]
		}

		inputs.PartialsPrompt.Segment = management.PartialsPromptSegment(inputs.Segment)

		currentPartials, err := cli.api.Prompt.ReadPartials(context.Background(), management.PartialsPromptSegment(inputs.Segment))
		if err != nil {
			return fmt.Errorf("failed to read current partials for prompt segment (%s): %w", ansi.Bold(inputs.Segment), err)
		}

		if err := promptPartialsFromInputs(inputs); err != nil {
			return err
		}

		if err := mergePartialsPrompts(&inputs.PartialsPrompt, currentPartials); err != nil {
			return fmt.Errorf("failed to merge initial partials with current partials for prompt segment (%s): %w", ansi.Bold(inputs.Segment), err)
		}

		if ok, _ := cmd.Flags().GetBool("no-input"); !ok {
			if err := editPartialsPrompt(cli, cmd, inputs); err != nil {
				return fmt.Errorf("failed to edit partials for prompt segment (%s): %w", ansi.Bold(inputs.Segment), err)
			}
		}

		if err := ansi.Waiting(func() error {
			return cli.api.Prompt.UpdatePartials(context.Background(), &inputs.PartialsPrompt)
		}); err != nil {
			return fmt.Errorf("failed to create partials for prompt segment (%s): %w", ansi.Bold(inputs.Segment), err)
		}

		partialsJSON, err := json.MarshalIndent(inputs.PartialsPrompt, "", "    ")
		if err != nil {
			return fmt.Errorf("failed to serialize the partials for prompt segment (%s) to json: %w", ansi.Bold(inputs.Segment), err)
		}

		cli.renderer.PromptsPartialsUpdate(string(partialsJSON), inputs.Segment)

		return nil
	}
}

func deletePromptsPartials(cli *cli, inputs *promptsPartialsInput) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := checkPromptsPartialsAvailable(cli); err != nil {
			return fmt.Errorf("error minimum requirements not met: %w", err)
		}

		if len(args) == 0 {
			if err := partialsPrompt.Pick(cmd, &inputs.Segment, promptsPartialsSegmentOptions); err != nil {
				return err
			}
		} else {
			if !validSegment(args[0]) {
				return errorInvalidSegment(args[0])
			}
			inputs.Segment = args[0]
		}

		inputs.PartialsPrompt.Segment = management.PartialsPromptSegment(inputs.Segment)

		if err := ansi.Waiting(func() error {
			return cli.api.Prompt.DeletePartials(context.Background(), &inputs.PartialsPrompt)
		}); err != nil {
			return fmt.Errorf("failed to delete partials for prompt segment (%s): %w", ansi.Bold(inputs.Segment), err)
		}

		partialsJSON, err := json.MarshalIndent(inputs.PartialsPrompt, "", "    ")
		if err != nil {
			return fmt.Errorf("failed to serialize the partials for prompt segment (%s) to json: %w", ansi.Bold(inputs.Segment), err)
		}

		cli.renderer.PromptsPartialsDelete(string(partialsJSON), inputs.Segment)

		return nil
	}
}

func promptsPartialsSegmentOptions(_ context.Context) (pickerOptions, error) {
	var opts pickerOptions
	for _, segment := range partialsSegmentTypes {
		s := string(segment)
		opts = append(opts, pickerOption{value: s, label: s})
	}
	return opts, nil
}

func validSegment(segment string) bool {
	s := management.PartialsPromptSegment(segment)
	for _, v := range partialsSegmentTypes {
		if s == v {
			return true
		}
	}
	return false
}

func errorInvalidSegment(segment string) error {
	return fmt.Errorf("error invalid prompt segment (%s) for partials", segment)
}

func promptPartialsFromInputs(inputs *promptsPartialsInput) error {
	inputs.PartialsPrompt.Segment = management.PartialsPromptSegment(inputs.Segment)

	if inputs.InputFile != "" {
		file, err := os.Open(inputs.InputFile)
		if err != nil {
			return fmt.Errorf("failed to open input file (%s): %w", ansi.Bold(inputs.InputFile), err)
		}
		defer func(file *os.File) {
			_ = file.Close()
		}(file)

		inputJSON, err := io.ReadAll(file)
		if err != nil {
			return fmt.Errorf("failed to read input file (%s): %w", ansi.Bold(inputs.InputFile), err)
		}

		if err := json.Unmarshal(inputJSON, &inputs.PartialsPrompt); err != nil {
			return fmt.Errorf("failed to deserialize input file (%s): %w", ansi.Bold(inputs.InputFile), err)
		}
	}

	return nil
}

func addCommonPromptPartialsFlags(cmd *cobra.Command, inputs *promptsPartialsInput) {
	// TODO: look for an automated way of adding flags.
	cmd.Flags().StringVar(&inputs.PartialsPrompt.FormContentStart, "form-content-start", "", "Content for the Form Content Start Partial")
	cmd.Flags().StringVar(&inputs.PartialsPrompt.FormContentEnd, "form-content-end", "", "Content for the Form Content End Partial")
	cmd.Flags().StringVar(&inputs.PartialsPrompt.FormFooterStart, "form-footer-start", "", "Content for the Form Footer Start Partial")
	cmd.Flags().StringVar(&inputs.PartialsPrompt.FormFooterEnd, "form-footer-end", "", "Content for the Form Footer End Partial")
	cmd.Flags().StringVar(&inputs.PartialsPrompt.SecondaryActionsStart, "secondary-actions-start", "", "Content for the Secondary Actions Start Partial")
	cmd.Flags().StringVar(&inputs.PartialsPrompt.SecondaryActionsEnd, "secondary-actions-end", "", "Content for the Secondary Actions End Partial")
}

func checkPromptsPartialsAvailable(cli *cli) error {
	domains, err := cli.api.CustomDomain.List(context.Background())
	if err != nil {
		return fmt.Errorf("custom domain error: %w", err)
	}
	if len(domains) < 1 {
		return fmt.Errorf("a custom domain must be created first")
	}
	if _, err := cli.api.Branding.Read(context.Background()); err != nil {
		return fmt.Errorf("branding must be enabled first: %w", err)
	}
	return nil
}

func editPartialsPrompt(cli *cli, cmd *cobra.Command, inputs *promptsPartialsInput) error {
	for {
		var editPartials string
		cli.renderer.Infof("Would you like edit your partials? (y/n)")
		if err := partialsPrompt.Ask(cmd, &editPartials); err != nil {
			return err
		}

		if editPartials == "n" || editPartials == "N" {
			return nil
		}

		cli.renderer.Infof("Choose a Partial to Edit")
		var partialKey string
		if err := partialsPrompt.Pick(cmd, &partialKey, func(ctx context.Context) (pickerOptions, error) {
			var opts pickerOptions
			for key, label := range partialsSegmentFields {
				opts = append(opts, pickerOption{value: key, label: label})
			}
			return opts, nil
		}); err != nil {
			return err
		}

		partialInput := getPartialsPromptsValueForKey(&inputs.PartialsPrompt, partialKey)
		fileName := fmt.Sprintf("%s-partials-prompt.json", inputs.Segment)
		if err := textBody.OpenEditor(cmd, &partialInput, partialInput, fileName, func() {
			cli.renderer.Infof(
				"%s Once you close the editor, the custom text will be saved. To cancel, press CTRL+C.",
				ansi.Faint("Hint:"),
			)
		}); err != nil {
			return fmt.Errorf("failed editing (%s) for partials prompt (%s): %w", partialKey, inputs.Segment, err)
		}

		if err := setPartialsPromptsValueForKey(&inputs.PartialsPrompt, partialKey, partialInput); err != nil {
			return fmt.Errorf("failed editing (%s) for partials prompt (%s): %w", partialKey, inputs.Segment, err)
		}
	}
}

func getPartialsPromptsValueForKey(input *management.PartialsPrompt, key string) string {
	partialsValue := reflect.ValueOf(input)
	partialsType := reflect.TypeOf(*input)

	for i := 0; i < partialsType.NumField(); i++ {
		tag := strings.Split(partialsType.Field(i).Tag.Get("json"), ",")[0]
		if tag == key {
			return partialsValue.Elem().Field(i).String()
		}
	}

	return ""
}

func setPartialsPromptsValueForKey(input *management.PartialsPrompt, key, value string) error {
	partialsValue := reflect.ValueOf(input)
	partialsType := reflect.TypeOf(*input)

	for i := 0; i < partialsType.NumField(); i++ {
		tag := strings.Split(partialsType.Field(i).Tag.Get("json"), ",")[0]
		if tag == key {
			partialsValue.Elem().Field(i).Set(reflect.ValueOf(value))
			return nil
		}
	}

	return fmt.Errorf("error key (%s) not found", ansi.Bold(key))
}

func mergePartialsPrompts(initial, current *management.PartialsPrompt) error {
	currentValue := reflect.ValueOf(current)
	currentType := reflect.TypeOf(*current)
	initialValue := reflect.ValueOf(initial)

	for i := 0; i < currentType.NumField(); i++ {
		cfield := currentValue.Elem().Field(i)
		ifield := initialValue.Elem().Field(i)
		if ifield.String() == "" {
			ifield.Set(cfield)
		}
	}

	return nil
}
