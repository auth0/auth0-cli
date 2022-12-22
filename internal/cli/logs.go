package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
)

var (
	logsFilter = Flag{
		Name:      "Filter",
		LongForm:  "filter",
		ShortForm: "f",
		Help:      "Filter in Lucene query syntax. See https://auth0.com/docs/logs/log-search-query-syntax for more details.",
	}

	logsNum = Flag{
		Name:      "Number of Entries",
		LongForm:  "number",
		ShortForm: "n",
		Help:      "Number of log entries to show.",
	}
)

func logsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View tenant logs",
		Long:  "View tenant logs.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listLogsCmd(cli))
	cmd.AddCommand(tailLogsCmd(cli))
	cmd.AddCommand(logStreamsCmd(cli))

	return cmd
}

func listLogsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Filter string
		Num    int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Show the tenant logs",
		Long:    "Display the tenant logs allowing to filter using Lucene query syntax.",
		Example: `  auth0 logs list
  auth0 logs list --filter "client_id:<client-id>"
  auth0 logs list --filter "client_name:<client-name>"
  auth0 logs list --filter "user_id:<user-id>"
  auth0 logs list --filter "user_name:<user-name>"
  auth0 logs list --filter "ip:<ip>"
  auth0 logs list --filter "type:f" # See the full list of type codes at https://auth0.com/docs/logs/log-event-type-codes
  auth0 logs ls -n 100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			list, err := getLatestLogs(cli, inputs.Num, inputs.Filter)
			if err != nil {
				return fmt.Errorf("An unexpected error occurred while getting logs: %v", err)
			}

			var logsCh chan []*management.Log
			cli.renderer.LogList(list, logsCh, !cli.debug)
			return nil
		},
	}

	logsFilter.RegisterString(cmd, &inputs.Filter, "")
	logsNum.RegisterInt(cmd, &inputs.Num, 100)

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func tailLogsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Filter string
		Num    int
	}

	cmd := &cobra.Command{
		Use:   "tail",
		Args:  cobra.MaximumNArgs(1),
		Short: "Tail the tenant logs",
		Long:  "Tail the tenant logs allowing to filter using Lucene query syntax.",
		Example: `  auth0 logs tail
  auth0 logs tail --filter "client_id:<client-id>"
  auth0 logs tail --filter "client_name:<client-name>"
  auth0 logs tail --filter "user_id:<user-id>"
  auth0 logs tail --filter "user_name:<user-name>"
  auth0 logs tail --filter "ip:<ip>"
  auth0 logs tail --filter "type:f" # See the full list of type codes at https://auth0.com/docs/logs/log-event-type-codes
  auth0 logs tail -n 100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			lastLogID := ""
			list, err := getLatestLogs(cli, inputs.Num, inputs.Filter)
			if err != nil {
				return fmt.Errorf("An unexpected error occurred while getting logs: %v", err)
			}

			// TODO(cyx): This is a hack for now to make the
			// streaming work faster.
			//
			// Create a `set` to detect duplicates clientside.
			set := make(map[string]struct{})
			list = dedupLogs(list, set)

			if len(list) > 0 {
				lastLogID = list[len(list)-1].GetLogID()
			}

			logsCh := make(chan []*management.Log)

			go func() {
				// This is pretty important and allows
				// us to close / terminate the command.
				defer close(logsCh)

				for {
					queryParams := []management.RequestOption{
						management.Query(fmt.Sprintf("log_id:[%s TO *]", lastLogID)),
						management.Parameter("page", "0"),
						management.Parameter("per_page", "100"),
						management.Parameter("sort", "date:-1"),
					}

					if inputs.Filter != "" {
						queryParams = append(queryParams, management.Query(inputs.Filter))
					}

					list, err = cli.api.Log.List(queryParams...)
					if err != nil {
						cli.renderer.Errorf("An unexpected error occurred while getting logs: %v", err)
						return
					}

					if len(list) > 0 {
						logsCh <- dedupLogs(list, set)
						lastLogID = list[len(list)-1].GetLogID()
					}

					if len(list) < 90 {
						// Not a lot is happening, sleep on it
						time.Sleep(1 * time.Second)
					}
				}
			}()

			cli.renderer.LogList(list, logsCh, !cli.debug)
			return nil
		},
	}

	logsFilter.RegisterString(cmd, &inputs.Filter, "")
	logsNum.RegisterInt(cmd, &inputs.Num, 100)

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func getLatestLogs(cli *cli, n int, filter string) ([]*management.Log, error) {
	page := 0
	perPage := n

	if perPage > 1000 {
		// Pagination max out at 1000 entries in total
		// https://auth0.com/docs/logs/retrieve-log-events-using-mgmt-api#limitations
		perPage = 1000
	}

	queryParams := []management.RequestOption{
		management.Parameter("sort", "date:-1"),
		management.Parameter("page", fmt.Sprintf("%d", page)),
		management.Parameter("per_page", fmt.Sprintf("%d", perPage))}

	if filter != "" {
		queryParams = append(queryParams, management.Query(filter))
	}

	return cli.api.Log.List(queryParams...)
}

func dedupLogs(list []*management.Log, set map[string]struct{}) []*management.Log {
	res := make([]*management.Log, 0, len(list))

	for _, l := range list {
		if _, ok := set[l.GetID()]; !ok {
			// It's not a duplicate, track it, and take it.
			set[l.GetID()] = struct{}{}
			res = append(res, l)
		}
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].GetDate().Before(res[j].GetDate())
	})

	return res
}
