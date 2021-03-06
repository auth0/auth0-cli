package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/auth0/auth0-cli/internal/auth0/actions"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

func getLatestLogs(cli *cli, n int) ([]*management.Log, error) {
	page := 0
	perPage := n

	if perPage > 1000 {
		// Pagination max out at 1000 entries in total
		// https://auth0.com/docs/logs/retrieve-log-events-using-mgmt-api#limitations
		perPage = 1000
	}

	return cli.api.Log.List(
		management.Parameter("sort", "date:-1"),
		management.Parameter("page", fmt.Sprintf("%d", page)),
		management.Parameter("per_page", fmt.Sprintf("%d", perPage)),
	)
}

func logsCmd(cli *cli) *cobra.Command {
	var flags struct {
		Num     int
		Follow  bool
		NoColor bool
	}

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show the tenant logs",
		Long: `auth0 logs
Show the tenant logs.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			lastLogID := ""
			list, err := getLatestLogs(cli, flags.Num)
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

			var logsCh chan []*management.Log
			if flags.Follow && lastLogID != "" {
				logsCh = make(chan []*management.Log)

				go func() {
					// This is pretty important and allows
					// us to close / terminate the command.
					defer close(logsCh)

					for {
						list, err = cli.api.Log.List(
							management.Query(fmt.Sprintf("log_id:[%s TO *]", lastLogID)),
							management.Parameter("page", "0"),
							management.Parameter("per_page", "100"),
							management.Parameter("sort", "date:-1"),
						)
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
			}

			// We create an execution API decorator which provides
			// a leaky bucket implementation for Read. This
			// protects us from being rate limited since we
			// potentially have an N+1 querying situation.
			actionExecutionAPI := actions.NewSampledExecutionAPI(
				cli.api.ActionExecution, time.Second,
			)

			cli.renderer.LogList(list, logsCh, actionExecutionAPI, flags.NoColor, cli.debug == false)
			return nil
		},
	}

	cmd.Flags().IntVarP(&flags.Num, "num-entries", "n", 100, "the number of log entries to print")
	cmd.Flags().BoolVarP(&flags.Follow, "follow", "f", false, "Specify if the logs should be streamed")
	cmd.Flags().BoolVar(&flags.NoColor, "no-color", false, "turn off colored print")

	return cmd
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
