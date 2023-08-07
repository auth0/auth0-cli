package cli

import (
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
		Name            string
		SplunkDomain    string
		SplunkToken     string
		SplunkPort      string
		SplunkVerifyTLS bool
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
  auth0 log streams create splunk --name <name> --domain <domain> --token <token> --port <port> --secure=false
  auth0 log streams create splunk -n <name> -d <domain> -t <token> -p <port> -s
  auth0 log streams create splunk -n mylogstream -d "demo.splunk.com" -t "12a34ab5-c6d7-8901-23ef-456b7c89d0c1" -p "8088" -s false --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logStreamName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := splunkDomain.Ask(cmd, &inputs.SplunkDomain, nil); err != nil {
				return err
			}

			if err := splunkToken.Ask(cmd, &inputs.SplunkToken, nil); err != nil {
				return err
			}

			if err := splunkPort.Ask(cmd, &inputs.SplunkPort, nil); err != nil {
				return err
			}

			if err := splunkVerifyTLS.AskBool(cmd, &inputs.SplunkVerifyTLS, nil); err != nil {
				return err
			}

			newLogStream := &management.LogStream{
				Name: &inputs.Name,
				Type: auth0.String(string(logStreamTypeSplunk)),
			}
			sink := &management.LogStreamSinkSplunk{
				Domain: &inputs.SplunkDomain,
				Token:  &inputs.SplunkToken,
				Secure: &inputs.SplunkVerifyTLS,
			}
			if inputs.SplunkPort != "" {
				sink.Port = &inputs.SplunkPort
			}
			newLogStream.Sink = sink

			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Create(cmd.Context(), newLogStream)
			}); err != nil {
				return fmt.Errorf("failed to create log stream: %v", err)
			}

			cli.renderer.LogStreamCreate(newLogStream)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	logStreamName.RegisterString(cmd, &inputs.Name, "")
	splunkDomain.RegisterString(cmd, &inputs.SplunkDomain, "")
	splunkToken.RegisterString(cmd, &inputs.SplunkToken, "")
	splunkPort.RegisterString(cmd, &inputs.SplunkPort, "")
	splunkVerifyTLS.RegisterBool(cmd, &inputs.SplunkVerifyTLS, false)

	return cmd
}

func updateLogStreamsSplunkCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID              string
		Name            string
		SplunkDomain    string
		SplunkToken     string
		SplunkPort      string
		SplunkVerifyTLS bool
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
  auth0 log streams update splunk <log-stream-id> --name <name> --domain <domain> --token <token> --port <port>
  auth0 log streams update splunk <log-stream-id> --name <name> --domain <domain> --token <token> --port <port> --secure=false
  auth0 log streams update splunk <log-stream-id> -n <name> -d <domain> -t <token> -p <port> -s
  auth0 log streams update splunk <log-stream-id> -n mylogstream -d "demo.splunk.com" -t "12a34ab5-c6d7-8901-23ef-456b7c89d0c1" -p "8088" -s=false --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logStreamID.Pick(cmd, &inputs.ID, cli.logStreamPickerOptionsByType(logStreamTypeSplunk))
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var oldLogStream *management.LogStream
			if err := ansi.Waiting(func() (err error) {
				oldLogStream, err = cli.api.LogStream.Read(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read log stream with ID %s: %w", inputs.ID, err)
			}

			if oldLogStream.GetType() != string(logStreamTypeSplunk) {
				return errInvalidLogStreamType(inputs.ID, oldLogStream.GetType(), string(logStreamTypeSplunk))
			}

			if err := logStreamName.AskU(cmd, &inputs.Name, oldLogStream.Name); err != nil {
				return err
			}

			splunkSink := oldLogStream.Sink.(*management.LogStreamSinkSplunk)

			if err := splunkDomain.AskU(cmd, &inputs.SplunkDomain, splunkSink.Domain); err != nil {
				return err
			}
			if err := splunkToken.AskU(cmd, &inputs.SplunkToken, splunkSink.Token); err != nil {
				return err
			}
			if err := splunkPort.AskU(cmd, &inputs.SplunkPort, splunkSink.Port); err != nil {
				return err
			}
			if err := splunkVerifyTLS.AskBoolU(cmd, &inputs.SplunkVerifyTLS, splunkSink.Secure); err != nil {
				return err
			}

			updatedLogStream := &management.LogStream{}
			if inputs.Name != "" {
				updatedLogStream.Name = &inputs.Name
			}
			if inputs.SplunkDomain != "" {
				splunkSink.Domain = &inputs.SplunkDomain
			}
			if inputs.SplunkToken != "" {
				splunkSink.Token = &inputs.SplunkToken
			}
			if inputs.SplunkPort != "" {
				splunkSink.Port = &inputs.SplunkPort
			}
			splunkSink.Secure = &inputs.SplunkVerifyTLS
			updatedLogStream.Sink = splunkSink

			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Update(cmd.Context(), oldLogStream.GetID(), updatedLogStream)
			}); err != nil {
				return fmt.Errorf("failed to update log stream with ID %s: %w", oldLogStream.GetID(), err)
			}

			cli.renderer.LogStreamUpdate(updatedLogStream)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	logStreamName.RegisterStringU(cmd, &inputs.Name, "")
	splunkDomain.RegisterStringU(cmd, &inputs.SplunkDomain, "")
	splunkToken.RegisterStringU(cmd, &inputs.SplunkToken, "")
	splunkPort.RegisterStringU(cmd, &inputs.SplunkPort, "")
	splunkVerifyTLS.RegisterBoolU(cmd, &inputs.SplunkVerifyTLS, false)

	return cmd
}
