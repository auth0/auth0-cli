package cli

import (
	"github.com/spf13/cobra"
	"time"

	"gopkg.in/auth0.v5/management"
)

func logsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "manage resources for logs.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(tailLogsCmd(cli))

	return cmd
}

func tailLogsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Tail your log events as they are happening",
		Long: `$ auth0 logs tail
Tail your logs as they are happening.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			list, err := cli.api.Log.List(management.Parameter("sort", "date:-1"))
			if err != nil {
				return err
			}
			var fromLogId = ""
			for {
				if len(list) > 0 {
					fromLogId = *list[len(list)-1].LogID
				}
				list, err := cli.api.Log.List(
					management.Parameter("from", fromLogId),
					management.Parameter("take", "100"),
				)
				if err != nil {
					return err
				}

				cli.renderer.LogList(list)
				time.Sleep(1 * time.Second)
			}
		},
	}

	return cmd
}
