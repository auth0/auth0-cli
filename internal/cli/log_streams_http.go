package cli

import (
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
		Name              string
		HTTPEndpoint      string
		HTTPContentType   string
		HTTPContentFormat string
		HTTPAuthorization string
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
  auth0 logs streams create http --name <name> --endpoint <endpoint> --type <type> --format <format> --authorization <authorization>
  auth0 logs streams create http -n <name> -e <endpoint> -t <type> -f <format> -a <authorization>
  auth0 logs streams create http -n mylogstream -e "https://example.com/webhook/logs" -t "application/json" -f "JSONLINES" -a "AKIAXXXXXXXXXXXXXXXX" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logStreamName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := httpEndpoint.Ask(cmd, &inputs.HTTPEndpoint, nil); err != nil {
				return err
			}

			if err := httpContentType.Ask(cmd, &inputs.HTTPContentType, nil); err != nil {
				return err
			}

			if err := httpContentFormat.Select(cmd, &inputs.HTTPContentFormat, httpContentFormatOptions, nil); err != nil {
				return err
			}

			if err := httpAuthorization.AskPassword(cmd, &inputs.HTTPAuthorization); err != nil {
				return err
			}

			newLogStream := &management.LogStream{
				Name: &inputs.Name,
				Type: auth0.String(string(logStreamTypeHTTP)),
			}
			sink := &management.LogStreamSinkHTTP{
				Endpoint: &inputs.HTTPEndpoint,
			}
			if inputs.HTTPAuthorization != "" {
				sink.Authorization = &inputs.HTTPAuthorization
			}
			if inputs.HTTPContentType != "" {
				sink.ContentType = &inputs.HTTPContentType
			}
			if inputs.HTTPContentFormat != "" {
				sink.ContentFormat = apiHTTPContentFormatFor(inputs.HTTPContentFormat)
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
	httpEndpoint.RegisterString(cmd, &inputs.HTTPEndpoint, "")
	httpContentType.RegisterString(cmd, &inputs.HTTPContentType, "")
	httpContentFormat.RegisterString(cmd, &inputs.HTTPContentFormat, "")
	httpAuthorization.RegisterString(cmd, &inputs.HTTPAuthorization, "")

	return cmd
}

func updateLogStreamsCustomWebhookCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID                string
		Name              string
		HTTPEndpoint      string
		HTTPContentType   string
		HTTPContentFormat string
		HTTPAuthorization string
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
  auth0 logs streams update http <log-stream-id> --name <name> --endpoint <endpoint> --type <type> --format <format>
  auth0 logs streams update http <log-stream-id> --name <name> --endpoint <endpoint> --type <type> --format <format> --authorization <authorization>
  auth0 logs streams update http <log-stream-id> -n <name> -e <endpoint> -t <type> -f <format> -a <authorization>
  auth0 logs streams update http <log-stream-id> -n mylogstream -e "https://example.com/webhook/logs" -t "application/json" -f "JSONLINES" -a "AKIAXXXXXXXXXXXXXXXX" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logStreamID.Pick(cmd, &inputs.ID, cli.logStreamPickerOptionsByType(logStreamTypeHTTP))
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

			if oldLogStream.GetType() != string(logStreamTypeHTTP) {
				return errInvalidLogStreamType(inputs.ID, oldLogStream.GetType(), string(logStreamTypeHTTP))
			}

			if err := logStreamName.AskU(cmd, &inputs.Name, oldLogStream.Name); err != nil {
				return err
			}

			httpSink := oldLogStream.Sink.(*management.LogStreamSinkHTTP)

			if err := httpEndpoint.AskU(cmd, &inputs.HTTPEndpoint, httpSink.Endpoint); err != nil {
				return err
			}
			if err := httpContentType.AskU(cmd, &inputs.HTTPContentType, httpSink.ContentType); err != nil {
				return err
			}
			if err := httpContentFormat.SelectU(cmd, &inputs.HTTPContentFormat, httpContentFormatOptions, httpSink.ContentFormat); err != nil {
				return err
			}
			if err := httpAuthorization.AskPasswordU(cmd, &inputs.HTTPAuthorization); err != nil {
				return err
			}

			updatedLogStream := &management.LogStream{}

			if inputs.Name != "" {
				updatedLogStream.Name = &inputs.Name
			}
			if inputs.HTTPEndpoint != "" {
				httpSink.Endpoint = &inputs.HTTPEndpoint
			}
			if inputs.HTTPAuthorization != "" {
				httpSink.Authorization = &inputs.HTTPAuthorization
			}
			if inputs.HTTPContentType != "" {
				httpSink.ContentType = &inputs.HTTPContentType
			}
			if inputs.HTTPContentFormat != "" {
				httpSink.ContentFormat = apiHTTPContentFormatFor(inputs.HTTPContentFormat)
			}

			updatedLogStream.Sink = httpSink

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
	httpEndpoint.RegisterStringU(cmd, &inputs.HTTPEndpoint, "")
	httpContentType.RegisterStringU(cmd, &inputs.HTTPContentType, "")
	httpContentFormat.RegisterStringU(cmd, &inputs.HTTPContentFormat, "")
	httpAuthorization.RegisterStringU(cmd, &inputs.HTTPAuthorization, "")

	return cmd
}

func apiHTTPContentFormatFor(v string) *string {
	return auth0.String(strings.ToUpper(v))
}
