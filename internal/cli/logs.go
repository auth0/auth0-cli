package cli

import (
	"fmt"
	"time"

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
	var numberOfLogs int
	var follow bool
	var noColor bool
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "show the tenant logs",
		Long: `$ auth0 logs
Show the tenant logs.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			lastLogID := ""
			list, err := getLatestLogs(cli, numberOfLogs)
			if err != nil {
				return err
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
			if follow {
				logsCh = make(chan []*management.Log)

				go func() {
					// This is pretty important and allows
					// us to close / terminate the command.
					defer close(logsCh)

					for {
						list, err = cli.api.Log.List(
							management.Query(fmt.Sprintf("log_id:[* TO %s]", lastLogID)),
							management.Parameter("page", "0"),
							management.Parameter("per_page", "100"),
							management.Parameter("sort", "date:-1"),
						)
						if err != nil {
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

			cli.renderer.LogList(list, logsCh, noColor)
			return nil
		},
	}

	cmd.Flags().IntVarP(&numberOfLogs, "num-entries", "n", 100, "the number of log entries to print")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Specify if the logs should be streamed.")
	cmd.Flags().BoolVarP(&noColor, "no-color", "", false, "turn off colored print")

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

	return res
}
