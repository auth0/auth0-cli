package cli

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/auth0/auth0-cli/internal/auth0"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
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

	logPicker = Flag{
		Name:      "Interactive picker option on rendered logs",
		LongForm:  "picker",
		ShortForm: "p",
		Help:      "Allows to toggle from list of logs and view a selected log in detail",
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
		Picker bool
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Show the tenant logs",
		Long:    "Display the tenant logs allowing to filter using Lucene query syntax.",
		Example: `  auth0 logs list
  auth0 logs list --filter "client_id:<client-id> --picker"
  auth0 logs list --filter "client_id:<client-id>"
  auth0 logs list --filter "client_name:<client-name>"
  auth0 logs list --filter "user_id:<user-id>"
  auth0 logs list --filter "user_name:<user-name>"
  auth0 logs list --filter "ip:<ip>"
  auth0 logs list --filter "type:f" # See the full list of type codes at https://auth0.com/docs/logs/log-event-type-codes
  auth0 logs ls -n 250 -p
  auth0 logs ls --json
  auth0 logs ls --json-compact
  auth0 logs ls --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Num < 1 || inputs.Num > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}
			logs, err := getLatestLogs(cmd.Context(), cli, inputs.Num, inputs.Filter)
			if err != nil {
				return fmt.Errorf("failed to list logs: %w", err)
			}

			hasFilter := inputs.Filter != ""
			if !inputs.Picker || len(logs) == 0 {
				cli.renderer.LogList(logs, !cli.debug, hasFilter)
			} else {
				var (
					selectedLogID string
					currentIndex  = auth0.Int(0)
				)
				for {
					selectedLogID = cli.renderer.LogPrompt(logs, currentIndex)

					logDetail, err := cli.api.Log.Read(cmd.Context(), selectedLogID)
					if err != nil {
						fmt.Println("Failed to fetch details:", err)
						continue
					}

					fmt.Println("\nDetailed Log:")
					cli.renderer.JSONResult(logDetail)

					if cli.renderer.QuitPrompt() {
						break
					}
				}
			}
			return nil
		},
	}

	logsFilter.RegisterString(cmd, &inputs.Filter, "")
	logsNum.RegisterInt(cmd, &inputs.Num, defaultPageSize)
	logPicker.RegisterBool(cmd, &inputs.Picker, false)

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

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
				return fmt.Errorf("failed to list logs: %w", err)
			}

			logsCh := make(chan []*management.Log)

			var lastLogID string
			if len(list) > 0 {
				lastLogID = list[0].GetLogID()
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

					if len(list) > 1 {
						logsCh <- dedupeLogs(list, set)
						lastLogID = list[0].GetLogID()
					}

					if len(list) < logsPerPageLimit {
						// Not a lot is happening, sleep on it.
						time.Sleep(2 * time.Second)
					}
				}
			}(lastLogID)

			cli.renderer.LogTail(list, logsCh, !cli.debug)
			return nil
		},
	}

	logsFilter.RegisterString(cmd, &inputs.Filter, "")
	logsNum.RegisterInt(cmd, &inputs.Num, defaultPageSize)

	return cmd
}

func getLatestLogs(ctx context.Context, cli *cli, numRequested int, filter string) ([]*management.Log, error) {
	page := 0
	logs := []*management.Log{}

	for {
		perPage := logsPerPageLimit
		if numRequested < (page+1)*logsPerPageLimit {
			perPage = numRequested % logsPerPageLimit
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

		page++
		if page == 10 || (page*logsPerPageLimit) >= numRequested {
			break
		}
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
