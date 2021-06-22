package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

const (
	logStreamTypeAmazonEventBridge = "eventbridge"
	logStreamTypeAzureEventGrid    = "eventgrid"
	logStreamTypeHTTP              = "http"
	logStreamTypeDatadog           = "datadog"
	logStreamTypeSplunk            = "splunk"
	logStreamTypeSumo              = "sumo"
)

var (
	logsID = Argument{
		Name: "Log stream ID",
		Help: "Log stream ID",
	}
	logStreamName = Flag{
		Name:         "Name",
		LongForm:     "name",
		ShortForm:    "n",
		Help:         "Name of the log stream.",
		AlwaysPrompt: true,
	}
	logStreamType = Flag{
		Name:       "Type",
		LongForm:   "type",
		ShortForm:  "t",
		Help:       "Type of the log stream. Possible values: http, eventbridge, eventgrid, datadog, splunk, sumo.",
		IsRequired: true,
	}
	typeOptions = []string{
		"HTTP",
		"EventBridge",
		"EventGrid",
		"DataDog",
		"Splunk",
		"Sumo",
	}
	httpEndpoint = Flag{
		Name:       "HTTP Endpoint",
		LongForm:   "http-endpoint",
		Help:       "HTTP endpoint.",
		IsRequired: true,
	}
	httpContentType = Flag{
		Name:         "HTTP Content Type",
		LongForm:     "http-type",
		Help:         "HTTP Content-Type header. Possible values: application/json.",
		AlwaysPrompt: true,
	}
	httpContentFormat = Flag{
		Name:         "HTTP Content Format",
		LongForm:     "http-format",
		Help:         "HTTP Content-Format header. Possible values: jsonlines, jsonarray, jsonobject.",
		AlwaysPrompt: true,
	}
	httpAuthorization = Flag{
		Name:         "HTTP Authorization",
		LongForm:     "http-auth",
		Help:         "HTTP Authorization header.",
		AlwaysPrompt: true,
	}
	awsAccountID = Flag{
		Name:       "AWS Account ID",
		LongForm:   "eventbridge-id",
		Help:       "Id of the AWS account.",
		IsRequired: true,
	}
	awsRegion = Flag{
		Name:       "AWS Region",
		LongForm:   "eventbridge-region",
		Help:       "The region in which eventbridge will be created.",
		IsRequired: true,
	}
	azureSubscriptionID = Flag{
		Name:       "Azure Subscription ID",
		LongForm:   "eventgrid-id",
		Help:       "Id of the Azure subscription.",
		IsRequired: true,
	}
	azureRegion = Flag{
		Name:       "Azure Region",
		LongForm:   "eventgrid-region",
		Help:       "The region in which the Azure subscription is hosted.",
		IsRequired: true,
	}
	azureResourceGroup = Flag{
		Name:       "Azure Resource Group",
		LongForm:   "eventgrid-group",
		Help:       "The name of the Azure resource group.",
		IsRequired: true,
	}
	datadogRegion = Flag{
		Name:     "Datadog Region",
		LongForm: "datadog-id",
		Help: "The region in which datadog dashboard is created.\n" +
			"if you are in the datadog EU site ('app.datadoghq.eu'), the Region should be EU otherwise it should be US.",
		IsRequired: true,
	}
	datadogApiKey = Flag{
		Name:       "Datadog API Key",
		LongForm:   "datadog-key",
		Help:       "Datadog API Key. To obtain a key, see the Datadog Authentication documentation (https://docs.datadoghq.com/api/latest/authentication).",
		IsRequired: true,
	}
	splunkDomain = Flag{
		Name:       "Splunk Domain",
		LongForm:   "splunk-domain",
		Help:       "The domain name of the splunk instance.",
		IsRequired: true,
	}
	splunkToken = Flag{
		Name:       "Splunk Token",
		LongForm:   "splunk-token",
		Help:       "Splunk event collector token.",
		IsRequired: true,
	}
	splunkPort = Flag{
		Name:     "Splunk Port",
		LongForm: "splunk-port",
		Help:     "The port of the HTTP event collector.",
	}
	splunkVerifyTLS = Flag{
		Name:     "Splunk Verify TLS",
		LongForm: "splunk-secure",
		Help:     "This should be set to 'false' when using self-signed certificates.",
	}
	sumoLogicSource = Flag{
		Name:       "Sumo Logic Source",
		LongForm:   "sumo-source",
		Help:       "Generated URL for your defined HTTP source in Sumo Logic.",
		IsRequired: true,
	}
)

func logStreamsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "streams",
		Short: "Manage resources for log streams",
		Long:  "manage resources for log streams.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listLogStreamsCmd(cli))
	cmd.AddCommand(showLogStreamCmd(cli))
	cmd.AddCommand(createLogStreamCmd(cli))
	cmd.AddCommand(updateLogStreamCmd(cli))
	cmd.AddCommand(deleteLogStreamCmd(cli))
	cmd.AddCommand(openLogStreamsCmd(cli))

	return cmd
}

func listLogStreamsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List all log streams",
		Long: `List your existing log streams. To create one try:
auth0 logs streams create`,
		Example: `auth0 logs streams list
auth0 logs streams ls`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list []*management.LogStream

			if err := ansi.Waiting(func() error {
				var err error
				list, err = cli.api.LogStream.List()
				return err
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			cli.renderer.LogStreamList(list)
			return nil
		},
	}

	return cmd
}

func showLogStreamCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID   string
		Type string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a log stream by Id",
		Long:  "Show a log stream by Id.",
		Example: `auth0 logs streams show
auth0 logs streams show <id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logsID.Ask(cmd, &inputs.ID)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			a := &management.LogStream{ID: &inputs.ID}

			if err := ansi.Waiting(func() error {
				var err error
				a, err = cli.api.LogStream.Read(inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load log stream: %w", err)
			}
			cli.renderer.LogStreamShow(a)
			return nil
		},
	}

	return cmd
}

func createLogStreamCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name                string
		Type                string
		HttpEndpoint        string
		HttpContentType     string
		HttpContentFormat   string
		HttpAuthorization   string
		SplunkDomain        string
		SplunkToken         string
		SplunkPort          string
		SplunkVerifyTLS     bool
		SumoLogicSource     string
		DatadogAPIKey       string
		DatadogRegion       string
		AwsAccountID        string
		AwsRegion           string
		AzureSubscriptionID string
		AzureRegion         string
		AzureResourceGroup  string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new log stream",
		Long:  "Create a new log stream.",
		Example: `auth0 logs streams create
auth0 logs streams create -n mylogstream -t http --http-type application/json --http-format JSONLINES --http-auth 1343434
auth0 logs streams create -n mydatadog -t datadog --datadog-key 9999999 --datadog-id us
auth0 logs streams create -n myeventbridge -t eventbridge --eventbridge-id 999999999999 --eventbridge-region us-east-1
auth0 logs streams create -n test-splunk -t splunk --splunk-domain demo.splunk.com --splunk-token 12a34ab5-c6d7-8901-23ef-456b7c89d0c1 --splunk-port 8080 --splunk-secure=true`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Prompt for log stream name
			if err := logStreamName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			// Prompt for log stream type
			if err := logStreamType.Select(cmd, &inputs.Type, typeOptions, nil); err != nil {
				return err
			}

			logIsHttp := logsTypeFor(inputs.Type) == logStreamTypeHTTP
			logIsSplunk := logsTypeFor(inputs.Type) == logStreamTypeSplunk
			logIsSumo := logsTypeFor(inputs.Type) == logStreamTypeSumo
			logIsDatadog := logsTypeFor(inputs.Type) == logStreamTypeDatadog
			logIsEventbridge := logsTypeFor(inputs.Type) == logStreamTypeAmazonEventBridge
			logIsEventgrid := logsTypeFor(inputs.Type) == logStreamTypeAzureEventGrid

			// Load values into a fresh log stream instance
			ls := &management.LogStream{
				Name: &inputs.Name,
				Type: auth0.String(logsTypeFor(inputs.Type)),
			}

			// Prompt for http sink details if type is http
			if logIsHttp {
				if err := httpEndpoint.Ask(cmd, &inputs.HttpEndpoint, nil); err != nil {
					return err
				}

				if err := httpContentType.Ask(cmd, &inputs.HttpContentType, nil); err != nil {
					return err
				}

				if err := httpContentFormat.Ask(cmd, &inputs.HttpContentFormat, nil); err != nil {
					return err
				}

				if err := httpAuthorization.Ask(cmd, &inputs.HttpAuthorization, nil); err != nil {
					return err
				}

				ls.Sink = &management.LogStreamSinkHTTP{
					Authorization: &inputs.HttpAuthorization,
					ContentType:   &inputs.HttpContentType,
					ContentFormat: apiHTTPContentFormatFor(inputs.HttpContentFormat),
					Endpoint:      &inputs.HttpEndpoint,
				}
			}

			// Prompt for splunk sink details if log stream type is splunk
			if logIsSplunk {
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

				ls.Sink = &management.LogStreamSinkSplunk{
					Domain: &inputs.SplunkDomain,
					Token:  &inputs.SplunkToken,
					Port:   &inputs.SplunkPort,
					Secure: &inputs.SplunkVerifyTLS,
				}
			}

			// Prompt for sumo sink details if log stream type is sumo
			if logIsSumo {
				if err := sumoLogicSource.Ask(cmd, &inputs.SumoLogicSource, nil); err != nil {
					return err
				}

				ls.Sink = &management.LogStreamSinkSumo{
					SourceAddress: &inputs.SumoLogicSource,
				}
			}

			// Prompt for datadog sink details if log stream type is datadog
			if logIsDatadog {
				if err := datadogApiKey.Ask(cmd, &inputs.DatadogAPIKey, nil); err != nil {
					return err
				}

				if err := datadogRegion.Ask(cmd, &inputs.DatadogRegion, nil); err != nil {
					return err
				}

				ls.Sink = &management.LogStreamSinkDatadog{
					Region: &inputs.DatadogRegion,
					APIKey: &inputs.DatadogAPIKey,
				}
			}

			// Prompt for eventbridge sink details if log stream type is eventbridge
			if logIsEventbridge {
				if err := awsAccountID.Ask(cmd, &inputs.AwsAccountID, nil); err != nil {
					return err
				}

				if err := awsRegion.Ask(cmd, &inputs.AwsRegion, nil); err != nil {
					return err
				}

				ls.Sink = &management.LogStreamSinkAmazonEventBridge{
					AccountID: &inputs.AwsAccountID,
					Region:    &inputs.AwsRegion,
				}
			}

			// Prompt for eventgrid sink details if log stream type is eventgrid
			if logIsEventgrid {
				if err := azureSubscriptionID.Ask(cmd, &inputs.AzureSubscriptionID, nil); err != nil {
					return err
				}

				if err := azureRegion.Ask(cmd, &inputs.AzureRegion, nil); err != nil {
					return err
				}

				if err := azureResourceGroup.Ask(cmd, &inputs.AzureResourceGroup, nil); err != nil {
					return err
				}

				ls.Sink = &management.LogStreamSinkAzureEventGrid{
					SubscriptionID: &inputs.AzureSubscriptionID,
					ResourceGroup:  &inputs.AzureResourceGroup,
					Region:         &inputs.AzureRegion,
				}
			}

			// Create log stream
			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Create(ls)
			}); err != nil {
				return fmt.Errorf("Unable to create log stream: %v", err)
			}

			// Render log stream creation specific view
			cli.renderer.LogStreamCreate(ls)
			return nil
		},
	}

	logStreamName.RegisterString(cmd, &inputs.Name, "")
	logStreamType.RegisterString(cmd, &inputs.Type, "")
	httpEndpoint.RegisterString(cmd, &inputs.HttpEndpoint, "")
	httpContentType.RegisterString(cmd, &inputs.HttpContentType, "")
	httpContentFormat.RegisterString(cmd, &inputs.HttpContentFormat, "")
	httpAuthorization.RegisterString(cmd, &inputs.HttpAuthorization, "")
	splunkDomain.RegisterString(cmd, &inputs.SplunkDomain, "")
	splunkToken.RegisterString(cmd, &inputs.SplunkToken, "")
	splunkPort.RegisterString(cmd, &inputs.SplunkPort, "")
	splunkVerifyTLS.RegisterBool(cmd, &inputs.SplunkVerifyTLS, false)
	sumoLogicSource.RegisterString(cmd, &inputs.SumoLogicSource, "")
	datadogApiKey.RegisterString(cmd, &inputs.DatadogAPIKey, "")
	datadogRegion.RegisterString(cmd, &inputs.DatadogRegion, "")
	awsAccountID.RegisterString(cmd, &inputs.AwsAccountID, "")
	awsRegion.RegisterString(cmd, &inputs.AwsRegion, "")
	azureSubscriptionID.RegisterString(cmd, &inputs.AzureSubscriptionID, "")
	azureRegion.RegisterString(cmd, &inputs.AzureRegion, "")
	azureResourceGroup.RegisterString(cmd, &inputs.AzureResourceGroup, "")

	return cmd
}

func updateLogStreamCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID                string
		Name              string
		Type              string
		HttpEndpoint      string
		HttpContentType   string
		HttpContentFormat string
		HttpAuthorization string
		HttpCustomHeaders []string
		SplunkDomain      string
		SplunkToken       string
		SplunkPort        string
		SplunkVerifyTLS   bool
		SumoLogicSource   string
		DatadogAPIKey     string
		DatadogRegion     string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a log stream",
		Long:  "Update a log stream.",
		Example: `auth0 logs streams update
auth0 logs streams update <id> --name mylogstream
auth0 logs streams update <id> -n mylogstream --type http
auth0 logs streams update <id> -n mylogstream -t http --http-type application/json --http-format JSONLINES
auth0 logs streams update <id> -n mydatadog -t datadog --datadog-key 9999999 --datadog-id us
auth0 logs streams update <id> -n myeventbridge -t eventbridge`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var current *management.LogStream

			if len(args) == 0 {
				err := logsID.Pick(cmd, &inputs.ID, cli.logStreamPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			// Load log stream by id
			if err := ansi.Waiting(func() error {
				var err error
				current, err = cli.api.LogStream.Read(inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load logstream: %w", err)
			}

			// Prompt for log stream name
			if err := logStreamName.AskU(cmd, &inputs.Name, current.Name); err != nil {
				return err
			}

			logIsHttp := auth0.StringValue(current.Type) == logStreamTypeHTTP
			logIsSplunk := auth0.StringValue(current.Type) == logStreamTypeSplunk
			logIsSumo := auth0.StringValue(current.Type) == logStreamTypeSumo
			logIsDatadog := auth0.StringValue(current.Type) == logStreamTypeDatadog

			// Load values into a fresh log stream instance
			ls := &management.LogStream{
				Status: current.Status,
			}

			// Prompt for datadog sink details if log stream type is datadog
			if logIsDatadog {
				s := &management.LogStreamSinkDatadog{}

				res := cli.getLogStreamSink(current.GetID())
				if err := json.Unmarshal([]byte(res), s); err != nil {
					fmt.Println(err)
				}

				if err := datadogApiKey.AskU(cmd, &inputs.DatadogAPIKey, s.APIKey); err != nil {
					return err
				}

				if err := datadogRegion.AskU(cmd, &inputs.DatadogRegion, s.Region); err != nil {
					return err
				}

				if len(inputs.DatadogAPIKey) > 0 {
					s.APIKey = &inputs.DatadogAPIKey
				}

				if len(inputs.DatadogRegion) > 0 {
					s.Region = &inputs.DatadogRegion
				}

				ls.Sink = &management.LogStreamSinkDatadog{
					Region: s.Region,
					APIKey: s.APIKey,
				}
			}

			// Prompt for sumo sink details if log stream type is sumo
			if logIsSumo {
				s := &management.LogStreamSinkSumo{}

				res := cli.getLogStreamSink(current.GetID())
				if err := json.Unmarshal([]byte(res), s); err != nil {
					fmt.Println(err)
				}

				if err := sumoLogicSource.AskU(cmd, &inputs.SumoLogicSource, s.SourceAddress); err != nil {
					return err
				}

				if len(inputs.SumoLogicSource) > 0 {
					s.SourceAddress = &inputs.SumoLogicSource
				}

				ls.Sink = &management.LogStreamSinkSumo{
					SourceAddress: s.SourceAddress,
				}
			}

			// Prompt for splunk sink details if log stream type is splunk
			if logIsSplunk {
				s := &management.LogStreamSinkSplunk{}

				res := cli.getLogStreamSink(current.GetID())
				if err := json.Unmarshal([]byte(res), s); err != nil {
					fmt.Println(err)
				}

				if err := splunkDomain.AskU(cmd, &inputs.SplunkDomain, s.Domain); err != nil {
					return err
				}

				if err := splunkToken.AskU(cmd, &inputs.SplunkToken, s.Token); err != nil {
					return err
				}

				if err := splunkPort.AskU(cmd, &inputs.SplunkPort, s.Port); err != nil {
					return err
				}

				if err := splunkVerifyTLS.AskBoolU(cmd, &inputs.SplunkVerifyTLS, s.Secure); err != nil {
					return err
				}

				if len(inputs.SplunkDomain) > 0 {
					s.Domain = &inputs.SplunkDomain
				}

				if len(inputs.SplunkToken) > 0 {
					s.Token = &inputs.SplunkToken
				}

				if len(inputs.SplunkPort) > 0 {
					s.Port = &inputs.SplunkPort
				}

				if !splunkVerifyTLS.IsSet(cmd) {
					s.Secure = auth0.Bool(inputs.SplunkVerifyTLS)
				}

				ls.Sink = &management.LogStreamSinkSplunk{
					Domain: s.Domain,
					Token:  s.Token,
					Port:   s.Port,
					Secure: s.Secure,
				}
			}

			// Prompt for http sink details if type is http
			if logIsHttp {
				s := &management.LogStreamSinkHTTP{}

				res := cli.getLogStreamSink(current.GetID())
				if err := json.Unmarshal([]byte(res), s); err != nil {
					fmt.Println(err)
				}

				if err := httpEndpoint.AskU(cmd, &inputs.HttpEndpoint, s.Endpoint); err != nil {
					return err
				}

				if err := httpContentType.AskU(cmd, &inputs.HttpContentType, s.ContentType); err != nil {
					return err
				}

				if err := httpContentFormat.AskU(cmd, &inputs.HttpContentFormat, s.ContentFormat); err != nil {
					return err
				}

				if err := httpAuthorization.AskU(cmd, &inputs.HttpAuthorization, s.Authorization); err != nil {
					return err
				}

				if len(inputs.HttpEndpoint) > 0 {
					s.Endpoint = &inputs.HttpEndpoint
				}

				if len(inputs.HttpContentType) > 0 {
					s.ContentType = &inputs.HttpContentType
				}

				if len(inputs.HttpContentFormat) > 0 {
					s.ContentFormat = apiHTTPContentFormatFor(inputs.HttpContentFormat)
				}

				if len(inputs.HttpAuthorization) > 0 {
					s.Authorization = &inputs.HttpAuthorization
				}

				ls.Sink = &management.LogStreamSinkHTTP{
					Authorization: s.Authorization,
					ContentType:   s.ContentType,
					ContentFormat: s.ContentFormat,
					Endpoint:      s.Endpoint,
				}
			}

			if len(inputs.Name) == 0 {
				ls.Name = current.Name
			} else {
				ls.Name = &inputs.Name
			}

			// Update a log stream
			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Update(current.GetID(), ls)
			}); err != nil {
				return fmt.Errorf("Unable to update log stream: %v", err)
			}

			// Render log stream update specific view
			cli.renderer.LogStreamUpdate(ls)
			return nil
		},
	}

	logStreamName.RegisterStringU(cmd, &inputs.Name, "")
	logStreamType.RegisterStringU(cmd, &inputs.Type, "")
	httpEndpoint.RegisterStringU(cmd, &inputs.HttpEndpoint, "")
	httpContentType.RegisterStringU(cmd, &inputs.HttpContentType, "")
	httpContentFormat.RegisterStringU(cmd, &inputs.HttpContentFormat, "")
	httpAuthorization.RegisterStringU(cmd, &inputs.HttpAuthorization, "")
	splunkDomain.RegisterStringU(cmd, &inputs.SplunkDomain, "")
	splunkToken.RegisterStringU(cmd, &inputs.SplunkToken, "")
	splunkPort.RegisterStringU(cmd, &inputs.SplunkPort, "")
	splunkVerifyTLS.RegisterBoolU(cmd, &inputs.SplunkVerifyTLS, false)
	sumoLogicSource.RegisterStringU(cmd, &inputs.SumoLogicSource, "")
	datadogApiKey.RegisterStringU(cmd, &inputs.DatadogAPIKey, "")
	datadogRegion.RegisterStringU(cmd, &inputs.DatadogRegion, "")

	return cmd
}

func deleteLogStreamCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "delete",
		Args:  cobra.MaximumNArgs(1),
		Short: "Delete a log stream",
		Long:  "Delete a log stream.",
		Example: `auth0 logs streams delete
auth0 logs streams delete <id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logsID.Pick(cmd, &inputs.ID, cli.logStreamPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.Spinner("Deleting Log Stream", func() error {
				_, err := cli.api.LogStream.Read(inputs.ID)

				if err != nil {
					return fmt.Errorf("Unable to delete log stream: %w", err)
				}

				return cli.api.LogStream.Delete(inputs.ID)
			})
		},
	}

	return cmd
}

func openLogStreamsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:     "open",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Open log stream settings page in the Auth0 Dashboard",
		Long:    "Open log stream settings page in the Auth0 Dashboard.",
		Example: "auth0 logs streams open <id>",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logsID.Pick(cmd, &inputs.ID, cli.logStreamPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			openManageURL(cli, cli.config.DefaultTenant, formatLogStreamSettingsPath(inputs.ID))
			return nil
		},
	}

	return cmd
}

func formatLogStreamSettingsPath(id string) string {
	if len(id) == 0 {
		return ""
	}
	return fmt.Sprintf("log-streams/%s/settings", id)
}

func (c *cli) logStreamPickerOptions() (pickerOptions, error) {
	list, err := c.api.LogStream.List()
	if err != nil {
		return nil, err
	}

	var opts pickerOptions

	for _, c := range list {
		value := c.GetID()
		label := fmt.Sprintf("%s %s", c.GetName(), ansi.Faint("("+value+")"))
		opts = append(opts, pickerOption{value: value, label: label})
	}

	if len(opts) == 0 {
		return nil, errNoRoles
	}

	return opts, nil
}

func apiHTTPContentFormatFor(v string) *string {
	return auth0.String(strings.ToUpper(v))
}

func logsTypeFor(v string) string {
	switch strings.ToLower(v) {
	case "http":
		return logStreamTypeHTTP
	case "eventbridge", "amazon eventbridge":
		return logStreamTypeAmazonEventBridge
	case "eventgrid", "azure eventgrid":
		return logStreamTypeAzureEventGrid
	case "datadog":
		return logStreamTypeDatadog
	case "splunk":
		return logStreamTypeSplunk
	case "sumo":
		return logStreamTypeSumo
	default:
		return v
	}
}

// getLogStreamSink
func (c *cli) getLogStreamSink(ID string) string {
	conn, err := c.api.LogStream.Read(ID)
	if err != nil {
		fmt.Println(err)
	}
	res := fmt.Sprintln(conn.Sink)

	return res
}
