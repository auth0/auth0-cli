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
		Long: "Manually block or unblock an IP address that was blocked via the Suspicious IP Throttling " +
			"due to multiple suspicious attempts.",
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
		Use:   "check",
		Args:  cobra.MaximumNArgs(1),
		Short: "Check IP address",
		Long: "Check if a given IP address is blocked via the Suspicious IP Throttling due to " +
			"multiple suspicious attempts.",
		Example: `  auth0 protection suspicious-ip-throttling ips check
  auth0 ap sit ips check <ip>
  auth0 ap sit ips check "178.178.178.178"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := ipAddress.Ask(cmd, &inputs.IP); err != nil {
					return err
				}
			} else {
				inputs.IP = args[0]
			}

			var isBlocked bool
			if err := ansi.Waiting(func() (err error) {
				isBlocked, err = cli.api.Anomaly.CheckIP(inputs.IP)
				return err
			}); err != nil {
				return fmt.Errorf("failed to check if IP %q is blocked: %w", inputs.IP, err)
			}

			cli.renderer.Heading("ip")

			if isBlocked {
				cli.renderer.Infof("The IP %s is blocked", inputs.IP)
				return nil
			}

			cli.renderer.Infof("The IP %s is not blocked.", inputs.IP)
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
		Use:   "unblock",
		Args:  cobra.MaximumNArgs(1),
		Short: "Unblock IP address",
		Long: "Unblock an IP address currently blocked by the Suspicious IP Throttling due to " +
			"multiple suspicious attempts.",
		Example: `  auth0 protection suspicious-ip-throttling ips unblock
  auth0 ap sit ips unblock <ip>
  auth0 ap sit ips unblock "178.178.178.178"`,
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
				return fmt.Errorf("failed to unblock IP %q: %w", inputs.IP, err)
			}

			cli.renderer.Heading("ip")
			cli.renderer.Infof("The IP %s was unblocked.", inputs.IP)

			return nil
		},
	}

	return cmd
}
