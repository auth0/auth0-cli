package display

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	coreagents "github.com/auth0/auth0-cli/internal/agents"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/text"
)

// AgentPreview writes the metadata of an agent that is about to be
// registered.
func (r *Renderer) AgentPreview(preview *coreagents.Preview) {
	r.poppyWrite(func(l Layout) string {
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

func describePoppyKeys(keys []coreagents.Key) string {
	algs := make([]string, len(keys))
	for i, key := range keys {
		algs[i] = key.Alg
	}
	slices.Sort(algs)

	return fmt.Sprintf("%d (%s)", len(keys), strings.Join(slices.Compact(algs), ", "))
}

// AgentRegisterPlan writes what registering an agent will do. The name is the one
// given for the agent record, which defaults to the name in its document.
func (r *Renderer) AgentRegisterPlan(preview *coreagents.Preview, name string, requireDPoP bool, grants []coreagents.Grant) {
	r.poppyWrite(func(l Layout) string {
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

// AgentRegistration writes a registered agent. A grant the agent app already had
// with other scopes than the ones requested is followed by what differs, and the
// command that sets the scopes.
func (r *Renderer) AgentRegistration(reg *coreagents.Registered) {
	r.poppyWrite(func(l Layout) string {
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

		return out + "\n" + poppyNext(l, [2]string{"auth0 agents show " + text.Quote(text.Line(cmp.Or(reg.AgentID, reg.URL))), "to check the agent."})
	})
}

// AgentRemovalStage says whether a removal is only described, about to be made,
// or done.
type AgentRemovalStage int

// The stages of a removal.
const (
	// AgentRemovalWould is a dry run.
	AgentRemovalWould AgentRemovalStage = iota
	// AgentRemovalWill is a removal that waits for approval.
	AgentRemovalWill
	// AgentRemovalDone is a removal that was made.
	AgentRemovalDone
)

// AgentRemoval writes what a removal removes or removed, what it keeps and how to
// remove the grants that remain. Once it is done, what was removed is marked as
// done.
func (r *Renderer) AgentRemoval(stage AgentRemovalStage, result *coreagents.RemoveResult) {
	removed, kept := "This would remove:", "This would keep:"

	switch stage {
	case AgentRemovalWill:
		removed, kept = "This will remove:", "This will keep:"
	case AgentRemovalDone:
		removed, kept = "Removed:", "Kept:"
	case AgentRemovalWould:
	}

	r.poppyWrite(func(l Layout) string {
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

			b.WriteString(ansi.Bold(heading))
			b.WriteString("\n")
			b.WriteString(l.entries(entries))
		}

		removedMark, keptMark := markPlain, markPlain
		if stage == AgentRemovalDone {
			removedMark, keptMark = markDone, markSkipped
		}

		list(removed, itemLines(result.Removed), removedMark)
		list(kept, itemLines(result.Kept), keptMark)

		if len(result.RemainingGrants) > 0 {
			if b.Len() > 0 {
				b.WriteString("\n")
			}

			b.WriteString(ansi.Bold("Other grants of the agent app stay. Remove them with:"))
			b.WriteString("\n")
			for _, grant := range result.RemainingGrants {
				b.WriteString(l.hang("  ", "  ", "auth0 client-grants delete "+text.Quote(text.Line(grant.ID))+"   ("+text.Line(grant.Audience)+")"))
			}
		}

		return b.String()
	})
}

// itemLines describes each thing a removal removes or keeps for a person.
func itemLines(items []coreagents.Item) []string {
	lines := make([]string, len(items))
	for i, item := range items {
		lines[i] = item.String()
	}

	return lines
}

// AgentRemovalGone warns that no agent app was found for a reference, which means
// it may have been removed already.
func (r *Renderer) AgentRemovalGone(ref string) {
	r.poppyWrite(func(l Layout) string {
		return l.entries([]poppyEntry{{mark: markWarning, value: "No agent app was found for " + text.Line(ref) + ". It may already be unregistered."}}) + "\n"
	})
}

// AgentRemoved writes a removal that was made, then what it does not do. The
// agent was gone when no app was found for it before.
func (r *Renderer) AgentRemoved(result *coreagents.RemoveResult, gone bool) {
	r.AgentRemoval(AgentRemovalDone, result)

	var notes []poppyEntry

	switch {
	case gone:
	case len(result.Kept) == 0:
		notes = append(notes, poppyEntry{mark: markWarning, value: "The refresh tokens of the app were revoked with it. Access tokens already issued stay valid until they expire."})
	default:
		notes = append(notes, poppyEntry{mark: markWarning, value: "The agent app was kept, so its refresh tokens are not revoked. To cut the agent off completely, unregister it without --poppy. Access tokens already issued stay valid until they expire."})
	}

	if len(result.Kept) == 0 && !gone {
		notes = append(notes, poppyEntry{mark: markWarning, value: "Registering the agent again creates a new app, so its users must sign in again."})
	}

	if len(notes) > 0 {
		r.poppyWrite(func(l Layout) string { return "\n" + l.entries(notes) })
	}
}
