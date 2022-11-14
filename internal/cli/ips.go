package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

var (
	ipAddress = Argument{
		Name: "IP",
		Help: "IP address to check.",
	}
)

func ipsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ips",
		Short: "Manage blocked IP addresses",
		Long:  "Manage blocked IP addresses.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(checkIPCmd(cli))
	cmd.AddCommand(unblockIPCmd(cli))

	return cmd
}

func checkIPCmd(cli *cli) *cobra.Command {
	var inputs struct {
		IP string
	}

	cmd := &cobra.Command{
		Use:     "check",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Check IP address",
		Long:    "Check whether a given IP address is blocked.",
		Example: "auth0 ips check <ip>",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := ipAddress.Ask(cmd, &inputs.IP); err != nil {
					return err
				}
			} else {
				inputs.IP = args[0]
			}

			var isBlocked bool

			if err := ansi.Waiting(func() error {
				var err error
				isBlocked, err = cli.api.Anomaly.CheckIP(inputs.IP)
				return err
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			cli.renderer.Heading("IP")

			if isBlocked {
				cli.renderer.Infof("The IP %s is blocked", inputs.IP)
			} else {
				cli.renderer.Infof("The IP %s is not blocked", inputs.IP)
			}

			return nil
		},
	}

	return cmd
}

func unblockIPCmd(cli *cli) *cobra.Command {
	var inputs struct {
		IP string
	}

	cmd := &cobra.Command{
		Use:     "unblock",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Unblock IP address",
		Long:    "Unblock an IP address which is currently blocked.",
		Example: "auth0 ips unblock <ip>",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := ipAddress.Ask(cmd, &inputs.IP); err != nil {
					return err
				}
			} else {
				inputs.IP = args[0]
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Anomaly.UnblockIP(inputs.IP)
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			cli.renderer.Heading("IP")
			cli.renderer.Infof("The IP %s was unblocked", inputs.IP)
			return nil
		},
	}

	return cmd
}
