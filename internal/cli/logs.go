package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

// Besides the limitation of 100 log events per request to retrieve logs,
// we may only paginate through up to 1000 search results.
// https://auth0.com/docs/logs/retrieve-log-events-using-mgmt-api#limitations
const logsPerPageLimit = 100

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
		Help:      "Number of log entries to show. Minimum 1, maximum 1000.",
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
  auth0 logs ls -n 250`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Num < 1 || inputs.Num > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}
			list, err := getLatestLogs(cmd.Context(), cli, inputs.Num, inputs.Filter)
			if err != nil {
				return fmt.Errorf("failed to get logs: %w", err)
			}

			hasFilter := inputs.Filter != ""
			cli.renderer.LogList(list, !cli.debug, hasFilter)
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
  auth0 logs tail -n 10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Num < 1 || inputs.Num > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}
			list, err := getLatestLogs(cmd.Context(), cli, inputs.Num, inputs.Filter)
			if err != nil {
				return fmt.Errorf("failed to get logs: %w", err)
			}

			logsCh := make(chan []*management.Log)

			var lastLogID string
			if len(list) > 0 {
				lastLogID = list[len(list)-1].GetLogID()
			}

			// Create a `set` to detect duplicates clientside.
			set := make(map[string]struct{})
			list = dedupeLogs(list, set)

			go func(lastLogID string) {
				defer close(logsCh)

				for {
					queryParams := []management.RequestOption{
						management.Parameter("page", "0"),
						management.Parameter("per_page", "100"),
						management.Parameter("sort", "date:-1"),
					}

					if lastLogID != "" {
						queryParams = append(queryParams, management.Query(fmt.Sprintf("log_id:[%s TO *]", lastLogID)))
					}

					if inputs.Filter != "" {
						queryParams = append(queryParams, management.Query(inputs.Filter))
					}

					list, err := cli.api.Log.List(cmd.Context(), queryParams...)
					if err != nil {
						cli.renderer.Errorf("Failed to get latest logs: %v", err)
						return
					}

					if len(list) > 0 {
						logsCh <- dedupeLogs(list, set)
						lastLogID = list[len(list)-1].GetLogID()
					}

					if len(list) < logsPerPageLimit {
						// Not a lot is happening, sleep on it.
						time.Sleep(time.Second)
					}
				}
			}(lastLogID)

			cli.renderer.LogTail(list, logsCh, !cli.debug)
			return nil
		},
	}

	logsFilter.RegisterString(cmd, &inputs.Filter, "")
	logsNum.RegisterInt(cmd, &inputs.Num, 100)

	return cmd
}

func getLatestLogs(ctx context.Context, cli *cli, numRequested int, filter string) ([]*management.Log, error) {
	page := 0
	logs := []*management.Log{}
	lastRequest := false

	for {
		perPage := logsPerPageLimit
		if ((page + 1) * logsPerPageLimit) > numRequested {
			perPage = numRequested % logsPerPageLimit
			if perPage == 0 {
				break
			}
			lastRequest = true
		}

		queryParams := []management.RequestOption{
			management.Parameter("sort", "date:-1"),
			management.Parameter("page", fmt.Sprintf("%d", page)),
			management.Parameter("per_page", fmt.Sprintf("%d", perPage))}

		if filter != "" {
			queryParams = append(queryParams, management.Query(filter))
		}

		res, err := cli.api.Log.List(ctx, queryParams...)
		if err != nil {
			return nil, err
		}
		logs = append(logs, res...)
		if lastRequest || page == 9 {
			break
		}

		page++
	}

	return logs, nil
}

func dedupeLogs(list []*management.Log, set map[string]struct{}) []*management.Log {
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
