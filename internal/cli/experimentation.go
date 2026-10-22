package cli

import (
	"context"
	"errors"

	managementcore "github.com/auth0/go-auth0/v3/management/core"
	"github.com/spf13/cobra"
)

const (
	// ExperimentationDataHelp is appended to the Long text of every
	// experimentation create/update command that accepts --schema and --data.
	experimentationDataHelp = "\n\nUse '--schema' to print the request payload schema and exit.\n" +
		"Use '--data' to supply the full JSON payload (validated against the schema before sending)."

		// ExperimentationQueryHelp is appended to the Long text of every
	// experimentation list command that accepts --schema and --query.
	experimentationQueryHelp = "\n\nUse '--schema' to see available query parameters.\n" +
		"Use '--query' to filter results via a JSON object (any API-supported parameter works immediately)."
)

// markQueryExclusive rejects combining --query with the individual filter
// flags, which the raw query would otherwise silently override. Each filter is
// paired with --query alone, so the filters still combine with each other.
func markQueryExclusive(cmd *cobra.Command, filterFlags ...string) {
	for _, name := range filterFlags {
		cmd.MarkFlagsMutuallyExclusive(listQueryFlag.LongForm, name)
	}
}

// collectPages returns the results of page and all the pages that follow it.
func collectPages[C comparable, T, R any](ctx context.Context, page *managementcore.Page[C, T, R]) ([]T, error) {
	var out []T
	for page != nil {
		out = append(out, page.Results...)

		next, err := page.GetNextPage(ctx)
		if errors.Is(err, managementcore.ErrNoPages) {
			break
		}
		if err != nil {
			return out, err
		}
		page = next
	}
	return out, nil
}

// experimentationCmd groups the Experiment Center resources (feature flags,
// segments, and experiments) under a single `auth0 experimentation` namespace,
// mirroring the Management API's experimentation grouping.
func experimentationCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "experimentation",
		Aliases: []string{"exp"},
		Short:   "Manage Experiment Center resources",
		Long: "Manage Auth0 Experiment Center resources: feature flags, segments, and experiments.\n\n" +
			"Feature flags define named parameters that experiments vary across user groups, segments group " +
			"users by matching rules, and experiments run A/B tests by tying a feature flag, its variations, " +
			"and traffic allocations together.",
	}

	cmd.SetUsageTemplate(namespaceUsageTemplate())
	cmd.AddCommand(featureFlagsCmd(cli))
	cmd.AddCommand(segmentsCmd(cli))
	cmd.AddCommand(experimentsCmd(cli))

	return cmd
}
