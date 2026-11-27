package display

import (
	"github.com/auth0/auth0-cli/internal/plugins"
)

// PluginProblem is one thing wrong with a plugin manifest: the JSON path of the
// field, and what is wrong with it.
type PluginProblem struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// PluginValidation is the result of checking a plugin manifest. Problems is empty,
// and not null, when the manifest is valid.
type PluginValidation struct {
	Valid    bool            `json:"valid"`
	Problems []PluginProblem `json:"problems"`
}

// PluginLinkResult is the result of linking a local build of a plugin. Banner is
// the line that every run of the linked plugin prints on stderr. ApproveCommand is
// only set when ApprovalRequired is true and there is a tenant.
type PluginLinkResult struct {
	ID               string `json:"id"`
	Version          string `json:"version"`
	Path             string `json:"path"`
	Bin              string `json:"bin"`
	Banner           string `json:"banner"`
	ApprovalRequired bool   `json:"approvalRequired"`
	ApproveCommand   string `json:"approveCommand,omitempty"`
}

type pluginProblemView struct {
	PluginProblem
}

func (v *pluginProblemView) AsTableHeader() []string {
	return []string{"Path", "Problem"}
}

func (v *pluginProblemView) AsTableRow() []string {
	return []string{plugins.SafeText(v.Path), plugins.SafeText(v.Message)}
}

func (v *pluginProblemView) Object() any {
	return &v.PluginProblem
}

// PluginValidation prints the result of checking a manifest. A person gets a
// table of the problems, or a line when there are none, and JSON gets the result
// as it is.
func (r *Renderer) PluginValidation(result PluginValidation) {
	switch {
	case r.Format == OutputFormatJSON:
		r.JSONResult(result)
	case r.Format == OutputFormatJSONCompact:
		r.JSONCompactResult(result)
	case result.Valid:
		r.Infof("The plugin manifest is valid.")
	default:
		problems := make([]View, 0, len(result.Problems))
		for _, problem := range result.Problems {
			problems = append(problems, &pluginProblemView{problem})
		}

		r.Results(problems)
	}
}

// PluginLinked prints the result of linking a plugin as JSON. In the other formats
// it prints nothing, because the command already told the person what it did.
func (r *Renderer) PluginLinked(result PluginLinkResult) {
	switch r.Format {
	case OutputFormatJSON:
		r.JSONResult(result)
	case OutputFormatJSONCompact:
		r.JSONCompactResult(result)
	}
}
