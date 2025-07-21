package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
)

var (
	httpEndpoint = Flag{
		Name:       "HTTP Endpoint",
		LongForm:   "endpoint",
		ShortForm:  "e",
		Help:       "The HTTP endpoint to send streaming logs to.",
		IsRequired: true,
	}
	httpContentType = Flag{
		Name:         "HTTP Content Type",
		LongForm:     "type",
		ShortForm:    "t",
		Help:         "The \"Content-Type\" header to send over HTTP. Common value is \"application/json\".",
		AlwaysPrompt: true,
	}
	httpContentFormat = Flag{
		Name:         "HTTP Content Format",
		LongForm:     "format",
		ShortForm:    "f",
		Help:         "The format of data sent over HTTP. Options are \"JSONLINES\", \"JSONARRAY\" or \"JSONOBJECT\"",
		AlwaysPrompt: true,
	}

	httpContentFormatOptions = []string{"JSONLINES", "JSONARRAY", "JSONOBJECT"}

	httpAuthorization = Flag{
		Name:         "HTTP Authorization",
		LongForm:     "authorization",
		ShortForm:    "a",
		Help:         "Sent in the HTTP \"Authorization\" header with each request.",
		AlwaysPrompt: true,
	}
)

func createLogStreamsCustomWebhookCmd(cli *cli) *cobra.Command {
	var inputs struct {
		name              string
		httpEndpoint      string
		httpContentType   string
		httpContentFormat string
		httpAuthorization string
		piiConfig         string
	}

	cmd := &cobra.Command{
		Use:   "http",
		Args:  cobra.NoArgs,
		Short: "Create a new Custom Webhook log stream",
		Long: "Specify a URL you'd like Auth0 to post events to.\n\n" +
			"To create interactively, use `auth0 logs streams create http` with no arguments.\n\n" +
			"To create non-interactively, supply the log stream name and other information through the flags.",
		Example: `  auth0 logs streams create http
  auth0 logs streams create http --name <name>
  auth0 logs streams create http --name <name> --endpoint <endpoint>
  auth0 logs streams create http --name <name> --endpoint <endpoint> --type <type>
  auth0 logs streams create http --name <name> --endpoint <endpoint> --type <type> --format <format>
  auth0 logs streams create http --name <name> --endpoint <endpoint> --type <type> --format <format> --pii-config '{"log_fields": ["first_name", "last_name"], "method": "hash", "algorithm": "xxhash"}''
  auth0 logs streams create http --name <name> --endpoint <endpoint> --type <type> --format <format> --authorization <authorization>
  auth0 logs streams create http -n <name> -e <endpoint> -t <type> -f <format> -a <authorization>
  auth0 logs streams create http -n mylogstream -e "https://example.com/webhook/logs" -t "application/json" -f "JSONLINES" -a "AKIAXXXXXXXXXXXXXXXX" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logStreamName.Ask(cmd, &inputs.name, nil); err != nil {
				return err
			}

			if err := httpEndpoint.Ask(cmd, &inputs.httpEndpoint, nil); err != nil {
				return err
			}

			if err := httpContentType.Ask(cmd, &inputs.httpContentType, nil); err != nil {
				return err
			}

			if err := httpContentFormat.Select(cmd, &inputs.httpContentFormat, httpContentFormatOptions, nil); err != nil {
				return err
			}

			if err := httpAuthorization.AskPassword(cmd, &inputs.httpAuthorization); err != nil {
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

			newLogStream := &management.LogStream{
				Name:      &inputs.name,
				Type:      auth0.String(string(logStreamTypeHTTP)),
				PIIConfig: piiConfig,
			}
			sink := &management.LogStreamSinkHTTP{
				Endpoint: &inputs.httpEndpoint,
			}
			if inputs.httpAuthorization != "" {
				sink.Authorization = &inputs.httpAuthorization
			}
			if inputs.httpContentType != "" {
				sink.ContentType = &inputs.httpContentType
			}
			if inputs.httpContentFormat != "" {
				sink.ContentFormat = apiHTTPContentFormatFor(inputs.httpContentFormat)
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
	logStreamName.RegisterString(cmd, &inputs.name, "")
	logStreamPIIConfig.RegisterString(cmd, &inputs.piiConfig, "{}")
	httpEndpoint.RegisterString(cmd, &inputs.httpEndpoint, "")
	httpContentType.RegisterString(cmd, &inputs.httpContentType, "")
	httpContentFormat.RegisterString(cmd, &inputs.httpContentFormat, "")
	httpAuthorization.RegisterString(cmd, &inputs.httpAuthorization, "")

	return cmd
}

func updateLogStreamsCustomWebhookCmd(cli *cli) *cobra.Command {
	var inputs struct {
		id                string
		name              string
		httpEndpoint      string
		httpContentType   string
		httpContentFormat string
		httpAuthorization string
		piiConfig         string
	}

	cmd := &cobra.Command{
		Use:   "http",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an existing Custom Webhook log stream",
		Long: "Specify a URL you'd like Auth0 to post events to.\n\n" +
			"To update interactively, use `auth0 logs streams create http` with no arguments.\n\n" +
			"To update non-interactively, supply the log stream name and other information through the flags.",
		Example: `  auth0 logs streams update http
  auth0 logs streams update http <log-stream-id> --name <name>
  auth0 logs streams update http <log-stream-id> --name <name> --endpoint <endpoint>
  auth0 logs streams update http <log-stream-id> --name <name> --endpoint <endpoint> --type <type>
  auth0 logs streams update http <log-stream-id> --name <name> --endpoint <endpoint> --type <type>  --pii-config '{"log_fields": ["first_name", "last_name"], "method": "mask", "algorithm": "xxhash"}'
  auth0 logs streams update http <log-stream-id> --name <name> --endpoint <endpoint> --type <type> --format <format>
  auth0 logs streams update http <log-stream-id> --name <name> --endpoint <endpoint> --type <type> --format <format> --authorization <authorization>
  auth0 logs streams update http <log-stream-id> -n <name> -e <endpoint> -t <type> -f <format> -a <authorization> -c null
  auth0 logs streams update http <log-stream-id> -n mylogstream -e "https://example.com/webhook/logs" -t "application/json" -f "JSONLINES" -a "AKIAXXXXXXXXXXXXXXXX" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logStreamID.Pick(cmd, &inputs.id, cli.logStreamPickerOptionsByType(logStreamTypeHTTP))
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

			if oldLogStream.GetType() != string(logStreamTypeHTTP) {
				return errInvalidLogStreamType(inputs.id, oldLogStream.GetType(), string(logStreamTypeHTTP))
			}

			if err := logStreamName.AskU(cmd, &inputs.name, oldLogStream.Name); err != nil {
				return err
			}

			existing, _ := json.Marshal(oldLogStream.GetPIIConfig())
			if err := logStreamPIIConfig.AskU(cmd, &inputs.piiConfig, auth0.String(string(existing))); err != nil {
				return err
			}

			httpSink := oldLogStream.Sink.(*management.LogStreamSinkHTTP)

			if err := httpEndpoint.AskU(cmd, &inputs.httpEndpoint, httpSink.Endpoint); err != nil {
				return err
			}
			if err := httpContentType.AskU(cmd, &inputs.httpContentType, httpSink.ContentType); err != nil {
				return err
			}
			if err := httpContentFormat.SelectU(cmd, &inputs.httpContentFormat, httpContentFormatOptions, httpSink.ContentFormat); err != nil {
				return err
			}
			if err := httpAuthorization.AskPasswordU(cmd, &inputs.httpAuthorization); err != nil {
				return err
			}

			updatedLogStream := &management.LogStream{}

			if inputs.name != "" {
				updatedLogStream.Name = &inputs.name
			}
			if inputs.httpEndpoint != "" {
				httpSink.Endpoint = &inputs.httpEndpoint
			}
			if inputs.httpAuthorization != "" {
				httpSink.Authorization = &inputs.httpAuthorization
			}
			if inputs.httpContentType != "" {
				httpSink.ContentType = &inputs.httpContentType
			}
			if inputs.httpContentFormat != "" {
				httpSink.ContentFormat = apiHTTPContentFormatFor(inputs.httpContentFormat)
			}

			updatedLogStream.Sink = httpSink

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
	httpEndpoint.RegisterStringU(cmd, &inputs.httpEndpoint, "")
	httpContentType.RegisterStringU(cmd, &inputs.httpContentType, "")
	httpContentFormat.RegisterStringU(cmd, &inputs.httpContentFormat, "")
	httpAuthorization.RegisterStringU(cmd, &inputs.httpAuthorization, "")

	return cmd
}

func apiHTTPContentFormatFor(v string) *string {
	return auth0.String(strings.ToUpper(v))
}
