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
	splunkDomain = Flag{
		Name:       "Splunk Domain",
		LongForm:   "domain",
		ShortForm:  "d",
		Help:       "The domain name of the splunk instance.",
		IsRequired: true,
	}
	splunkToken = Flag{
		Name:       "Splunk Token",
		LongForm:   "token",
		ShortForm:  "t",
		Help:       "Splunk event collector token.",
		IsRequired: true,
	}
	splunkPort = Flag{
		Name:      "Splunk Port",
		LongForm:  "port",
		ShortForm: "p",
		Help:      "The port of the HTTP event collector.",
	}
	splunkVerifyTLS = Flag{
		Name:      "Splunk Verify TLS",
		LongForm:  "secure",
		ShortForm: "s",
		Help:      "This should be set to 'false' when using self-signed certificates.",
	}
)

func createLogStreamsSplunkCmd(cli *cli) *cobra.Command {
	var inputs struct {
		name            string
		splunkDomain    string
		splunkToken     string
		splunkPort      string
		splunkVerifyTLS bool
		piiConfig       string
		filters         string
	}

	cmd := &cobra.Command{
		Use:   "splunk",
		Args:  cobra.NoArgs,
		Short: "Create a new Splunk log stream",
		Long: "Monitor real-time logs and display log analytics.\n\n" +
			"To create interactively, use `auth0 logs streams create splunk` with no arguments.\n\n" +
			"To create non-interactively, supply the log stream name and other information through the flags.",
		Example: `  auth0 log streams create splunk
  auth0 log streams create splunk --name <name>
  auth0 log streams create splunk --name <name> --domain <domain>
  auth0 log streams create splunk --name <name> --domain <domain> --token <token>
  auth0 log streams create splunk --name <name> --domain <domain> --token <token> --port <port>
  auth0 log streams create splunk --name <name> --domain <domain> --token <token> --port <port> --filters '[{"type":"category","name":"auth.login.fail"},{"type":"category","name":"auth.signup.fail"}]'
  auth0 log streams create splunk --name <name> --domain <domain> --token <token> --port <port> --pii-config '{"log_fields": ["first_name", "last_name"], "method": "hash", "algorithm": "xxhash"}'
  auth0 log streams create splunk --name <name> --domain <domain> --token <token> --port <port> --secure=false
  auth0 log streams create splunk -n <name> -d <domain> -t <token> -p <port> -s
  auth0 log streams create splunk -n mylogstream -d "demo.splunk.com" -t "12a34ab5-c6d7-8901-23ef-456b7c89d0c1" -p "8088" -s false --json
  auth0 log streams create splunk -n mylogstream -d "demo.splunk.com" -t "12a34ab5-c6d7-8901-23ef-456b7c89d0c1" -p "8088" -s false --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logStreamName.Ask(cmd, &inputs.name, nil); err != nil {
				return err
			}

			if err := splunkDomain.Ask(cmd, &inputs.splunkDomain, nil); err != nil {
				return err
			}

			if err := splunkToken.Ask(cmd, &inputs.splunkToken, nil); err != nil {
				return err
			}

			if err := splunkPort.Ask(cmd, &inputs.splunkPort, nil); err != nil {
				return err
			}

			if err := splunkVerifyTLS.AskBool(cmd, &inputs.splunkVerifyTLS, nil); err != nil {
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
				Name:      &inputs.name,
				Type:      auth0.String(string(logStreamTypeSplunk)),
				PIIConfig: piiConfig,
				Filters:   filters,
			}
			sink := &management.LogStreamSinkSplunk{
				Domain: &inputs.splunkDomain,
				Token:  &inputs.splunkToken,
				Secure: &inputs.splunkVerifyTLS,
			}
			if inputs.splunkPort != "" {
				sink.Port = &inputs.splunkPort
			}
			newLogStream.Sink = sink

			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Create(cmd.Context(), newLogStream)
			}); err != nil {
				return fmt.Errorf("failed to create log stream: %w", err)
			}

			return cli.renderer.LogStreamCreate(newLogStream)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	logStreamName.RegisterString(cmd, &inputs.name, "")
	logStreamFilters.RegisterString(cmd, &inputs.filters, "[]")
	logStreamPIIConfig.RegisterString(cmd, &inputs.piiConfig, "{}")
	splunkDomain.RegisterString(cmd, &inputs.splunkDomain, "")
	splunkToken.RegisterString(cmd, &inputs.splunkToken, "")
	splunkPort.RegisterString(cmd, &inputs.splunkPort, "")
	splunkVerifyTLS.RegisterBool(cmd, &inputs.splunkVerifyTLS, false)

	return cmd
}

func updateLogStreamsSplunkCmd(cli *cli) *cobra.Command {
	var inputs struct {
		id              string
		name            string
		splunkDomain    string
		splunkToken     string
		splunkPort      string
		splunkVerifyTLS bool
		piiConfig       string
		filters         string
	}

	cmd := &cobra.Command{
		Use:   "splunk",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an existing Splunk log stream",
		Long: "Monitor real-time logs and display log analytics.\n\n" +
			"To update interactively, use `auth0 logs streams create splunk` with no arguments.\n\n" +
			"To update non-interactively, supply the log stream name and other information through the flags.",
		Example: `  auth0 log streams update splunk
  auth0 log streams update splunk <log-stream-id> --name <name>
  auth0 log streams update splunk <log-stream-id> --name <name> --domain <domain>
  auth0 log streams update splunk <log-stream-id> --name <name> --domain <domain> --token <token>
  auth0 log streams update splunk <log-stream-id> --name <name> --domain <domain> --token <token> --pii-config '{"log_fields": ["first_name", "last_name"], "method": "mask", "algorithm": "xxhash"}'
  auth0 log streams update splunk <log-stream-id> --name <name> --domain <domain> --token <token> --filters '[{"type":"category","name":"user.fail"},{"type":"category","name":"scim.event"}]'
  auth0 log streams update splunk <log-stream-id> --name <name> --domain <domain> --token <token> --port <port>
  auth0 log streams update splunk <log-stream-id> --name <name> --domain <domain> --token <token> --port <port> --secure=false
  auth0 log streams update splunk <log-stream-id> -n <name> -d <domain> -t <token> -p <port> -s -c null
  auth0 log streams update splunk <log-stream-id> -n mylogstream -d "demo.splunk.com" -t "12a34ab5-c6d7-8901-23ef-456b7c89d0c1" -p "8088" -s=false --json
  auth0 log streams update splunk <log-stream-id> -n mylogstream -d "demo.splunk.com" -t "12a34ab5-c6d7-8901-23ef-456b7c89d0c1" -p "8088" -s=false --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logStreamID.Pick(cmd, &inputs.id, cli.logStreamPickerOptionsByType(logStreamTypeSplunk))
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
				return fmt.Errorf("failed to read log stream with ID %q: %w", inputs.id, err)
			}

			if oldLogStream.GetType() != string(logStreamTypeSplunk) {
				return errInvalidLogStreamType(inputs.id, oldLogStream.GetType(), string(logStreamTypeSplunk))
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

			splunkSink := oldLogStream.Sink.(*management.LogStreamSinkSplunk)

			if err := splunkDomain.AskU(cmd, &inputs.splunkDomain, splunkSink.Domain); err != nil {
				return err
			}
			if err := splunkToken.AskU(cmd, &inputs.splunkToken, splunkSink.Token); err != nil {
				return err
			}
			if err := splunkPort.AskU(cmd, &inputs.splunkPort, splunkSink.Port); err != nil {
				return err
			}
			if !splunkVerifyTLS.IsSet(cmd) {
				inputs.splunkVerifyTLS = splunkSink.GetSecure()
			}
			if err := splunkVerifyTLS.AskBoolU(cmd, &inputs.splunkVerifyTLS, splunkSink.Secure); err != nil {
				return err
			}

			updatedLogStream := &management.LogStream{
				PIIConfig: oldLogStream.GetPIIConfig(),
			}
			if inputs.name != "" {
				updatedLogStream.Name = &inputs.name
			}
			if inputs.splunkDomain != "" {
				splunkSink.Domain = &inputs.splunkDomain
			}
			if inputs.splunkToken != "" {
				splunkSink.Token = &inputs.splunkToken
			}
			if inputs.splunkPort != "" {
				splunkSink.Port = &inputs.splunkPort
			}
			splunkSink.Secure = &inputs.splunkVerifyTLS
			updatedLogStream.Sink = splunkSink

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
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	logStreamName.RegisterStringU(cmd, &inputs.name, "")
	logStreamPIIConfig.RegisterStringU(cmd, &inputs.piiConfig, "{}")
	logStreamFilters.RegisterStringU(cmd, &inputs.filters, "[]")
	splunkDomain.RegisterStringU(cmd, &inputs.splunkDomain, "")
	splunkToken.RegisterStringU(cmd, &inputs.splunkToken, "")
	splunkPort.RegisterStringU(cmd, &inputs.splunkPort, "")
	splunkVerifyTLS.RegisterBoolU(cmd, &inputs.splunkVerifyTLS, false)

	return cmd
}
