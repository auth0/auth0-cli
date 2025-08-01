package cli

import (
	"encoding/json"
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
)

var (
	sumoLogicSource = Flag{
		Name:       "Sumo Logic Source",
		LongForm:   "source",
		ShortForm:  "s",
		Help:       "Generated URL for your defined HTTP source in Sumo Logic.",
		IsRequired: true,
	}
)

func createLogStreamsSumoLogicCmd(cli *cli) *cobra.Command {
	var inputs struct {
		mame            string
		sumoLogicSource string
		piiConfig       string
		filters         string
	}

	cmd := &cobra.Command{
		Use:   "sumo",
		Args:  cobra.NoArgs,
		Short: "Create a new Sumo Logic log stream",
		Long: "Visualize logs and detect threats faster with security insights.\n\n" +
			"To create interactively, use `auth0 logs streams create sumo` with no arguments.\n\n" +
			"To create non-interactively, supply the log stream name and other information through the flags.",
		Example: `  auth0 logs streams create sumo
  auth0 logs streams create sumo --name <name>
  auth0 logs streams create sumo --name <name> --source <source>
  auth0 logs streams create sumo --name <name> --source <source> --filters '[{"type":"category","name":"auth.login.fail"},{"type":"category","name":"auth.signup.fail"}]'
  auth0 logs streams create sumo --name <name> --source <source> --pii-config '{"log_fields": ["first_name", "last_name"], "method": "hash", "algorithm": "xxhash"}'
  auth0 logs streams create sumo -n <name> -s <source>
  auth0 logs streams create sumo -n "mylogstream" -s "demo.sumo.com" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logStreamName.Ask(cmd, &inputs.mame, nil); err != nil {
				return err
			}

			if err := sumoLogicSource.Ask(cmd, &inputs.sumoLogicSource, nil); err != nil {
				return err
			}

			var piiConfig *management.LogStreamPiiConfig
			if err := logStreamPIIConfig.Ask(cmd, &inputs.piiConfig, auth0.String("{}")); err != nil {
				return err
			}

			if inputs.piiConfig != "{}" {
				if err := json.Unmarshal([]byte(inputs.piiConfig), &piiConfig); err != nil {
					return fmt.Errorf("provider: %s credentials invalid JSON: %w", inputs.piiConfig, err)
				}
			}

			var filters *[]map[string]string
			if err := logStreamFilters.Ask(cmd, &filters, auth0.String("[]")); err != nil {
				return err
			}

			if inputs.filters != "[]" {
				if err := json.Unmarshal([]byte(inputs.filters), &filters); err != nil {
					return fmt.Errorf("provider: %s filters invalid JSON: %w", inputs.filters, err)
				}
			}

			newLogStream := &management.LogStream{
				Name: &inputs.mame,
				Type: auth0.String(string(logStreamTypeSumo)),
				Sink: &management.LogStreamSinkSumo{
					SourceAddress: &inputs.sumoLogicSource,
				},
				PIIConfig: piiConfig,
				Filters:   filters,
			}

			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Create(cmd.Context(), newLogStream)
			}); err != nil {
				return fmt.Errorf("failed to create log stream: %w", err)
			}

			return cli.renderer.LogStreamCreate(newLogStream)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	logStreamName.RegisterString(cmd, &inputs.mame, "")
	logStreamPIIConfig.RegisterString(cmd, &inputs.piiConfig, "{}")
	logStreamFilters.RegisterString(cmd, &inputs.filters, "[]")
	sumoLogicSource.RegisterString(cmd, &inputs.sumoLogicSource, "")

	return cmd
}

func updateLogStreamsSumoLogicCmd(cli *cli) *cobra.Command {
	var inputs struct {
		id              string
		name            string
		sumoLogicSource string
		piiConfig       string
		filters         string
	}

	cmd := &cobra.Command{
		Use:   "sumo",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an existing Sumo Logic log stream",
		Long: "Visualize logs and detect threats faster with security insights.\n\n" +
			"To update interactively, use `auth0 logs streams create sumo` with no arguments.\n\n" +
			"To update non-interactively, supply the log stream name and other information through the flags.",
		Example: `  auth0 logs streams update sumo
  auth0 logs streams update sumo <log-stream-id> --name <name>
  auth0 logs streams update sumo <log-stream-id> --name <name> --source <source>
  auth0 logs streams update sumo <log-stream-id> --name <name> --source <source> --filters '[{"type":"category","name":"user.fail"},{"type":"category","name":"scim.event"}]'
  auth0 logs streams update sumo <log-stream-id> --name <name> --source <source>  --pii-config '{"log_fields": ["first_name", "last_name"], "method": "mask", "algorithm": "xxhash"}'
  auth0 logs streams update sumo <log-stream-id> -n <name> -s <source> -c null
  auth0 logs streams update sumo <log-stream-id> -n "mylogstream" -s "demo.sumo.com" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logStreamID.Pick(cmd, &inputs.id, cli.logStreamPickerOptionsByType(logStreamTypeSumo))
				if err != nil {
					return err
				}
			} else {
				inputs.id = args[0]
			}

			var oldLogStream *management.LogStream
			if err := ansi.Waiting(func() (err error) {
				oldLogStream, err = cli.api.LogStream.Read(cmd.Context(), inputs.id)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read log stream with id %q: %w", inputs.id, err)
			}

			if oldLogStream.GetType() != string(logStreamTypeSumo) {
				return errInvalidLogStreamType(inputs.id, oldLogStream.GetType(), string(logStreamTypeSumo))
			}

			if err := logStreamName.AskU(cmd, &inputs.name, oldLogStream.Name); err != nil {
				return err
			}

			existingConfig, _ := json.Marshal(oldLogStream.GetPIIConfig())
			if err := logStreamPIIConfig.AskU(cmd, &inputs.piiConfig, auth0.String(string(existingConfig))); err != nil {
				return err
			}

			existingFilters, _ := json.Marshal(oldLogStream.GetFilters())
			if err := logStreamFilters.AskU(cmd, &inputs.filters, auth0.String(string(existingFilters))); err != nil {
				return err
			}

			sumoSink := oldLogStream.Sink.(*management.LogStreamSinkSumo)
			if err := sumoLogicSource.AskU(cmd, &inputs.sumoLogicSource, sumoSink.SourceAddress); err != nil {
				return err
			}

			updatedLogStream := &management.LogStream{
				PIIConfig: oldLogStream.GetPIIConfig(),
			}
			if inputs.name != "" {
				updatedLogStream.Name = &inputs.name
			}
			if inputs.sumoLogicSource != "" {
				sumoSink.SourceAddress = &inputs.sumoLogicSource
			}
			updatedLogStream.Sink = sumoSink

			if inputs.piiConfig != "{}" {
				var piiConfig *management.LogStreamPiiConfig
				if err := json.Unmarshal([]byte(inputs.piiConfig), &piiConfig); err != nil {
					return fmt.Errorf("provider: %s credentials invalid JSON: %w", inputs.piiConfig, err)
				}
				updatedLogStream.PIIConfig = piiConfig
			}

			if inputs.filters != "[]" {
				var filters *[]map[string]string
				if err := json.Unmarshal([]byte(inputs.filters), &filters); err != nil {
					return fmt.Errorf("provider: %s filters invalid JSON: %w", inputs.filters, err)
				}
				updatedLogStream.Filters = filters
			}

			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Update(cmd.Context(), oldLogStream.GetID(), updatedLogStream)
			}); err != nil {
				return fmt.Errorf("failed to update log stream with ID %q: %w", oldLogStream.GetID(), err)
			}

			return cli.renderer.LogStreamUpdate(updatedLogStream)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	logStreamName.RegisterStringU(cmd, &inputs.name, "")
	logStreamPIIConfig.RegisterStringU(cmd, &inputs.piiConfig, "{}")
	logStreamFilters.RegisterStringU(cmd, &inputs.filters, "[]")
	sumoLogicSource.RegisterStringU(cmd, &inputs.sumoLogicSource, "")

	return cmd
}
