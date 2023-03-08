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
		HttpEndpoint      string
		HttpContentType   string
		HttpContentFormat string
		HttpAuthorization string
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

			if err := httpEndpoint.Ask(cmd, &inputs.HttpEndpoint, nil); err != nil {
				return err
			}

			if err := httpContentType.Ask(cmd, &inputs.HttpContentType, nil); err != nil {
				return err
			}

			if err := httpContentFormat.Select(cmd, &inputs.HttpContentFormat, httpContentFormatOptions, nil); err != nil {
				return err
			}

			if err := httpAuthorization.AskPassword(cmd, &inputs.HttpAuthorization, nil); err != nil {
				return err
			}

			newLogStream := &management.LogStream{
				Name: &inputs.Name,
				Type: auth0.String(string(logStreamTypeHTTP)),
			}
			sink := &management.LogStreamSinkHTTP{
				Endpoint: &inputs.HttpEndpoint,
			}
			if inputs.HttpAuthorization != "" {
				sink.Authorization = &inputs.HttpAuthorization
			}
			if inputs.HttpContentType != "" {
				sink.ContentType = &inputs.HttpContentType
			}
			if inputs.HttpContentFormat != "" {
				sink.ContentFormat = apiHTTPContentFormatFor(inputs.HttpContentFormat)
			}
			newLogStream.Sink = sink

			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Create(newLogStream)
			}); err != nil {
				return fmt.Errorf("failed to create log stream: %v", err)
			}

			cli.renderer.LogStreamCreate(newLogStream)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	logStreamName.RegisterString(cmd, &inputs.Name, "")
	httpEndpoint.RegisterString(cmd, &inputs.HttpEndpoint, "")
	httpContentType.RegisterString(cmd, &inputs.HttpContentType, "")
	httpContentFormat.RegisterString(cmd, &inputs.HttpContentFormat, "")
	httpAuthorization.RegisterString(cmd, &inputs.HttpAuthorization, "")

	return cmd
}

func updateLogStreamsCustomWebhookCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID                string
		Name              string
		HttpEndpoint      string
		HttpContentType   string
		HttpContentFormat string
		HttpAuthorization string
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
				oldLogStream, err = cli.api.LogStream.Read(inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read log stream with ID %s: %w", inputs.ID, err)
			}

			if oldLogStream.GetType() != string(logStreamTypeHTTP) {
				return fmt.Errorf(
					"the log stream with ID %q is of type %q instead of http, "+
						"use 'auth0 logs streams update %s' to update it instead",
					inputs.ID,
					oldLogStream.GetType(),
					oldLogStream.GetType(),
				)
			}

			if err := logStreamName.AskU(cmd, &inputs.Name, oldLogStream.Name); err != nil {
				return err
			}

			httpSink := oldLogStream.Sink.(*management.LogStreamSinkHTTP)

			if err := httpEndpoint.AskU(cmd, &inputs.HttpEndpoint, httpSink.Endpoint); err != nil {
				return err
			}
			if err := httpContentType.AskU(cmd, &inputs.HttpContentType, httpSink.ContentType); err != nil {
				return err
			}
			if err := httpContentFormat.SelectU(cmd, &inputs.HttpContentFormat, httpContentFormatOptions, httpSink.ContentFormat); err != nil {
				return err
			}
			if err := httpAuthorization.AskPasswordU(cmd, &inputs.HttpAuthorization, httpSink.Authorization); err != nil {
				return err
			}

			updatedLogStream := &management.LogStream{}

			if inputs.Name != "" {
				updatedLogStream.Name = &inputs.Name
			}
			if inputs.HttpEndpoint != "" {
				httpSink.Endpoint = &inputs.HttpEndpoint
			}
			if inputs.HttpAuthorization != "" {
				httpSink.Authorization = &inputs.HttpAuthorization
			}
			if inputs.HttpContentType != "" {
				httpSink.ContentType = &inputs.HttpContentType
			}
			if inputs.HttpContentFormat != "" {
				httpSink.ContentFormat = apiHTTPContentFormatFor(inputs.HttpContentFormat)
			}

			updatedLogStream.Sink = httpSink

			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Update(oldLogStream.GetID(), updatedLogStream)
			}); err != nil {
				return fmt.Errorf("failed to update log stream with ID %s: %w", oldLogStream.GetID(), err)
			}

			cli.renderer.LogStreamUpdate(updatedLogStream)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	logStreamName.RegisterStringU(cmd, &inputs.Name, "")
	httpEndpoint.RegisterStringU(cmd, &inputs.HttpEndpoint, "")
	httpContentType.RegisterStringU(cmd, &inputs.HttpContentType, "")
	httpContentFormat.RegisterStringU(cmd, &inputs.HttpContentFormat, "")
	httpAuthorization.RegisterStringU(cmd, &inputs.HttpAuthorization, "")

	return cmd
}

func apiHTTPContentFormatFor(v string) *string {
	return auth0.String(strings.ToUpper(v))
}
