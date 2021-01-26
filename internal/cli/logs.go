package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

func getLatestLogs(cli *cli, n int) (result []*management.Log, err error) {
	var list []*management.Log
	page := 0
	perPage := 100
	var count int
	if count = n; n > 1000 {
		// Pagination max out at 1000 entries in total
		// https://auth0.com/docs/logs/retrieve-log-events-using-mgmt-api#limitations
		count = 1000
	}
	if perPage > count {
		perPage = count
	}

	err = ansi.Spinner("Getting logs", func() error {
		for count > len(result) {
			var err error
			list, err = cli.api.Log.List(
				management.Parameter("sort", "date:-1"),
				management.Parameter("page", fmt.Sprintf("%d", page)),
				management.Parameter("per_page", fmt.Sprintf("%d", perPage)),
			)
			if err != nil {
				return err
			}

			sort.Slice(list, func(i, j int) bool {
				return list[i].GetDate().Before(list[j].GetDate())
			})
			result = append(list, result...)
			if len(list) < perPage {
				// We've got all
				break
			}
			page++
		}
		return err
	})

	return
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
			if len(list) > 0 {
				lastLogID = list[len(list)-1].GetLogID()
				cli.renderer.LogList(list, noColor)
			}
			if follow {
				for {
					list, err = cli.api.Log.List(
						management.Parameter("from", lastLogID),
						management.Parameter("take", "100"),
					)
					if err != nil {
						return err
					}
					if len(list) > 0 {
						cli.renderer.LogList(list, noColor)
						lastLogID = list[len(list)-1].GetLogID()
					}
					if len(list) < 90 {
						// Not a lot is happening, sleep on it
						time.Sleep(1 * time.Second)
					}
				}
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&numberOfLogs, "num-entries", "n", 100, "the number of log entries to print")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Specify if the logs should be streamed.")
	cmd.Flags().BoolVarP(&noColor, "no-color", "", false, "turn off colored print")

	return cmd
}
