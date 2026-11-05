package display

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/poppy/agents"
	"github.com/auth0/auth0-cli/internal/poppy/text"
)

// poppyAgentView renders an agent as a row of the agents table.
type poppyAgentView struct {
	agents.Agent
}

func (v poppyAgentView) AsTableHeader() []string {
	return []string{"AGENT ID", "NAME", "URL", "METADATA", "APIS"}
}

func (v poppyAgentView) AsTableRow() []string {
	return []string{
		ansi.Faint(cmp.Or(text.Line(v.AgentID), "-")), text.Line(v.Name), text.Line(v.URL), text.Line(v.MetadataStatus),
		text.Line(describePoppyAPIs(v.APIs, ", ")),
	}
}

func (v poppyAgentView) Object() interface{} { return v.Agent }

// poppyAgentDetailView renders one agent as key-value rows.
type poppyAgentDetailView struct {
	*agents.Detail
}

func (v poppyAgentDetailView) AsTableHeader() []string { return []string{} }

func (v poppyAgentDetailView) AsTableRow() []string { return []string{} }

func (v poppyAgentDetailView) Object() interface{} { return v.Detail }

func (v poppyAgentDetailView) KeyValues() [][]string {
	dpop := "unknown"
	if v.DPoPRequired.Known {
		dpop = map[bool]string{true: "required", false: "not required"}[v.DPoPRequired.Value]
	}

	document := "unknown"
	if v.MetadataCheck.Valid.Known {
		document = "valid"
		if !v.MetadataCheck.Valid.Value {
			document = "invalid: " + text.Line(strings.Join(v.MetadataCheck.Violations, "; "))
		}
	}

	keys := "unknown"
	if v.KeysCurrent.Known {
		keys = map[bool]string{true: "current", false: "changed since registration"}[v.KeysCurrent.Value]
	}

	return [][]string{
		{"AGENT ID", ansi.Faint(cmp.Or(text.Line(v.AgentID), "-"))},
		{"NAME", text.Line(v.Name)},
		{"URL", text.Line(v.URL)},
		{"AGENT APP", text.Line(v.ClientID)},
		{"METADATA", text.Line(v.MetadataStatus)},
		{"DPOP", dpop},
		{"APIS", text.Escape(describePoppyAPIs(v.APIs, "\n"))},
		{"DOCUMENT", document},
		{"KEYS", keys},
	}
}

func describePoppyAPIs(apis agents.Maybe[[]agents.Grant], separator string) string {
	if !apis.Known {
		return "unknown"
	}

	if len(apis.Value) == 0 {
		return "-"
	}

	audiences := make([]string, len(apis.Value))
	for i, grant := range apis.Value {
		audiences[i] = grant.Audience
	}

	return strings.Join(audiences, separator)
}

// PoppyAgentList writes the registered agents as a table, or says there are none.
func (r *Renderer) PoppyAgentList(list []agents.Agent) {
	resource := "agents"

	r.Heading(fmt.Sprintf("%s (%d)", resource, len(list)))

	if len(list) == 0 {
		r.EmptyState(resource, "Register one with: auth0 poppy agents register <agent-url>")
		return
	}

	views := make([]View, len(list))
	for i, agent := range list {
		views[i] = poppyAgentView{agent}
	}

	r.Results(views)
}

// PoppyAgentShow writes the details of an agent, then what could not be checked
// and the warning that its keys changed.
func (r *Renderer) PoppyAgentShow(detail *agents.Detail) {
	r.Heading("agent")
	r.Result(poppyAgentDetailView{detail})

	// The notes and the key status are fields of the JSON, so they are not
	// repeated on stderr.
	if r.suppressDecorations() {
		return
	}

	for _, note := range detail.Notes {
		r.Warnf("%s", text.Line(note))
	}

	if detail.KeysCurrent.Known && !detail.KeysCurrent.Value {
		r.Warnf("The agent's keys changed since it was registered. Run `auth0 poppy agents register %s` again.", text.Line(detail.URL))
	}
}

// PoppyAgentPreview writes the metadata of an agent that is about to be
// registered.
func (r *Renderer) PoppyAgentPreview(preview *agents.Preview) {
	r.poppyWrite(func(l PoppyLayout) string {
		entries := []poppyEntry{
			{mark: markPlain, label: "name", value: orDash(text.Line(preview.Name))},
			{mark: markPlain, label: "redirect URLs", value: orDash(text.Line(strings.Join(preview.RedirectURLs, ", ")))},
			{mark: markPlain, label: "keys", value: describePoppyKeys(preview.Keys)},
		}

		if preview.LogoURI != "" {
			entries = append(entries, poppyEntry{mark: markPlain, label: "logo", value: text.Line(preview.LogoURI)})
		}

		for _, warning := range preview.Warnings {
			entries = append(entries, poppyEntry{mark: markWarning, value: text.Line(warning)})
		}

		if preview.ClientID != "" {
			entries = append(entries, poppyEntry{mark: markChanged, value: fmt.Sprintf("An agent app already exists for this URL (%s). It will be updated.", text.Line(preview.ClientID))})
		}

		return l.hang("Agent metadata  ", ansi.Bold("Agent metadata")+"  ", orDash(text.Line(preview.URL))) + l.entries(entries)
	})
}

// orDash stands for a value that is empty.
func orDash(s string) string { return cmp.Or(s, "-") }

func describePoppyKeys(keys []agents.Key) string {
	algs := make([]string, len(keys))
	for i, key := range keys {
		algs[i] = key.Alg
	}
	slices.Sort(algs)

	return fmt.Sprintf("%d (%s)", len(keys), strings.Join(slices.Compact(algs), ", "))
}

// PoppyRegisterPlan writes what registering an agent will do. The name is the one
// given for the agent record, which defaults to the name in its document.
func (r *Renderer) PoppyRegisterPlan(preview *agents.Preview, name string, requireDPoP bool, grants []agents.Grant) {
	r.poppyWrite(func(l PoppyLayout) string {
		app := poppyEntry{mark: markAdded, label: "Agent app", value: "create, for " + text.Line(preview.URL)}
		if preview.ClientID != "" {
			app = poppyEntry{mark: markChanged, label: "Agent app", value: "update, for " + text.Line(preview.URL)}
		}

		entries := []poppyEntry{app}

		if requireDPoP {
			entries = append(entries, poppyEntry{mark: markAdded, label: "DPoP", value: "required on the agent app"})
		}

		entries = append(entries, poppyEntry{mark: markAdded, label: "Agent record", value: fmt.Sprintf("%q", text.Line(cmp.Or(name, preview.Name)))})

		for _, grant := range grants {
			entries = append(entries, poppyEntry{
				mark:  markAdded,
				label: "Grant",
				value: text.Line(grant.Audience) + ": " + text.Line(strings.Join(grant.Scopes, " ")) + "  (skipped if the agent app already has a grant)",
			})
		}

		return "\n" + ansi.Bold("This will") + "\n" + l.entries(entries)
	})
}

// PoppyRegistration writes a registered agent. A grant the agent app already had
// with other scopes than the ones requested is followed by what differs, and the
// command that sets the scopes.
func (r *Renderer) PoppyRegistration(reg *agents.Registration) {
	r.poppyWrite(func(l PoppyLayout) string {
		app := poppyEntry{mark: markDone, label: "Agent app", value: "created  " + text.Line(reg.ClientID)}
		if !reg.AppCreated {
			app = poppyEntry{mark: markChanged, label: "Agent app", value: "updated  " + text.Line(reg.ClientID)}
		}

		entries := []poppyEntry{app}

		if reg.DPoPRequired {
			entries = append(entries, poppyEntry{mark: markDone, label: "DPoP", value: "required"})
		}

		record := poppyEntry{mark: markDone, label: "Agent record", value: "exists  " + text.Line(reg.AgentID)}
		if reg.AgentCreated {
			record.value = fmt.Sprintf("created  %s (in tokens as %s)", text.Line(reg.AgentID), text.Line(reg.URL))
		}

		entries = append(entries, record)

		var differences []poppyEntry

		for _, grant := range reg.Grants {
			value := text.Line(grant.Audience) + ": " + text.Line(strings.Join(grant.Scopes, " "))
			if !grant.Created {
				value += "  (already granted)"
			}

			entries = append(entries, poppyEntry{mark: markDone, label: "Grant", value: value})

			if grant.Note != "" {
				differences = append(differences, poppyEntry{mark: markWarning, value: text.Line(grant.Note)})
			}

			if grant.Fix != "" {
				differences = append(differences, poppyEntry{mark: markPlain, label: "FIX", value: text.Line(grant.Fix)})
			}
		}

		out := "\n" + l.entries(entries)
		if len(differences) > 0 {
			out += l.entries(differences)
		}

		return out + "\n" + poppyNext(l, [2]string{"auth0 poppy agents show " + text.Quote(text.Line(reg.URL)), "to check the agent."})
	})
}

// PoppyRemovalStage says whether a removal is only described, about to be made,
// or done.
type PoppyRemovalStage int

// The stages of a removal.
const (
	// PoppyRemovalWould is a dry run.
	PoppyRemovalWould PoppyRemovalStage = iota
	// PoppyRemovalWill is a removal that waits for approval.
	PoppyRemovalWill
	// PoppyRemovalDone is a removal that was made.
	PoppyRemovalDone
)

// PoppyRemoval writes what a removal removes or removed, what it keeps and how to
// remove the grants that remain. Once it is done, what was removed is marked as
// done.
func (r *Renderer) PoppyRemoval(stage PoppyRemovalStage, result *agents.RemoveResult) {
	removed, kept := "This would remove:", "This would keep:"

	switch stage {
	case PoppyRemovalWill:
		removed, kept = "This will remove:", "This will keep:"
	case PoppyRemovalDone:
		removed, kept = "Removed:", "Kept:"
	case PoppyRemovalWould:
	}

	r.poppyWrite(func(l PoppyLayout) string {
		var b strings.Builder

		list := func(heading string, items []string, mark poppyMark) {
			if len(items) == 0 {
				return
			}

			entries := make([]poppyEntry, len(items))
			for i, item := range items {
				entries[i] = poppyEntry{mark: mark, value: text.Line(item)}
			}

			if b.Len() > 0 {
				b.WriteString("\n")
			}

			b.WriteString(ansi.Bold(heading) + "\n" + l.entries(entries))
		}

		removedMark, keptMark := markPlain, markPlain
		if stage == PoppyRemovalDone {
			removedMark, keptMark = markDone, markSkipped
		}

		list(removed, result.Removed, removedMark)
		list(kept, result.Kept, keptMark)

		if len(result.RemainingGrants) > 0 {
			if b.Len() > 0 {
				b.WriteString("\n")
			}

			b.WriteString(ansi.Bold("Other grants of the agent app stay. Remove them with:") + "\n")
			for _, grant := range result.RemainingGrants {
				b.WriteString(l.hang("  ", "  ", "auth0 client-grants delete "+text.Quote(text.Line(grant.ID))+"   ("+text.Line(grant.Audience)+")"))
			}
		}

		return b.String()
	})
}

// PoppyRemovalGone warns that no agent app was found for a reference, which means
// it may have been removed already.
func (r *Renderer) PoppyRemovalGone(ref string) {
	r.poppyWrite(func(l PoppyLayout) string {
		return l.entries([]poppyEntry{{mark: markWarning, value: "No agent app was found for " + text.Line(ref) + ". It may already be removed."}}) + "\n"
	})
}

// PoppyRemoved writes a removal that was made, then what it does not do. The
// agent was gone when no app was found for it before.
func (r *Renderer) PoppyRemoved(result *agents.RemoveResult, gone bool) {
	r.PoppyRemoval(PoppyRemovalDone, result)

	var notes []poppyEntry
	if !gone {
		notes = append(notes, poppyEntry{mark: markWarning, value: "This command does not revoke tokens already issued: access tokens stay valid until they expire."})
	}

	if len(result.Kept) == 0 && !gone {
		notes = append(notes, poppyEntry{mark: markWarning, value: "Registering the agent again creates a new app, so its users must sign in again."})
	}

	if len(notes) > 0 {
		r.poppyWrite(func(l PoppyLayout) string { return "\n" + l.entries(notes) })
	}
}
