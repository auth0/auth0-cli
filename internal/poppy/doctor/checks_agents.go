package doctor

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	coreagents "github.com/auth0/auth0-cli/internal/agents"
	poppyagents "github.com/auth0/auth0-cli/internal/agents/poppy"
)

// checkAgents runs D11 to D13 on every Poppy agent. They are skipped when the API
// part is off, as they need a Poppy API. D13 also reports the agents that have no
// grant at all, which D11 and D12 leave out, as they check what an agent was
// registered with.
func (r *runner) checkAgents(ctx context.Context, t *Tenant, apis []coreagents.API, apisErr error, apiOff bool) (d11, d12, d13 collector) {
	ids := []struct {
		id   string
		spec string
		col  *collector
	}{{"D11", specAgentApp, &d11}, {"D12", specAgentApp, &d12}, {"D13", "", &d13}}

	each := func(record func(id, spec string, c *collector)) {
		for _, check := range ids {
			record(check.id, check.spec, check.col)
		}
	}

	if apiOff {
		each(func(id, spec string, c *collector) { c.skip(id+".api_off", spec, SkipAPIOff, messageAPIOff) })
		return d11, d12, d13
	}

	if errors.Is(apisErr, poppyagents.ErrNoPoppyAPI) {
		each(func(id, spec string, c *collector) {
			c.skip(id+".no_poppy_api", spec, SkipNotAvailable, "no Poppy API")
		})
		return d11, d12, d13
	}

	var list []coreagents.GrantedAgent
	err := apisErr
	if err == nil {
		list, err = t.Agents.List(ctx, true)
	}
	if err != nil {
		each(func(id, spec string, c *collector) { r.unreadable(c, id, spec, "the agents", scopeAgents, err) })
		return d11, d12, d13
	}

	if len(list) == 0 {
		each(func(id, spec string, c *collector) {
			c.skip(id+".no_agents", spec, SkipNotAvailable, "the tenant has no agent")
		})
		return d11, d12, d13
	}

	granted := 0
	for _, agent := range list {
		from := [3]int{len(d11.results), len(d12.results), len(d13.results)}

		if agent.MayHaveGrants() {
			granted++
			r.checkAgentApp(agent, &d11)
			r.checkAgentKeys(ctx, t, agent, &d12)
		}
		r.checkAgentGrants(agent, apis, &d13)

		d11.forAgent(from[0], agent)
		d12.forAgent(from[1], agent)
		d13.forAgent(from[2], agent)
	}

	if granted == 0 {
		d11.skip("D11.no_agent_grants", specAgentApp, SkipNotAvailable, "no agent has a grant to a Poppy API")
		d12.skip("D12.no_agent_grants", specAgentApp, SkipNotAvailable, "no agent has a grant to a Poppy API")
	}

	return d11, d12, d13
}

// agentName is how the messages of the results name an agent: by its name when it
// has one, as its URL is long. The result keeps the URL in its agent field.
func agentName(agent coreagents.GrantedAgent) string {
	return cmp.Or(agent.Name, agent.URL, agent.AgentID)
}

// forAgent marks the results recorded since the given count as about the agent.
func (c *collector) forAgent(from int, agent coreagents.GrantedAgent) {
	ref := &AgentRef{ID: agent.AgentID, URL: agent.URL, Name: agent.Name}

	for i := from; i < len(c.results); i++ {
		c.results[i].Agent = ref
	}
}

// registerFix is the fix that registers an agent again, with the APIs given. An
// agent without a URL cannot be registered from here, so the fix is a sentence.
func (r *runner) registerFix(agent coreagents.GrantedAgent, apis ...string) fix {
	if agent.URL == "" {
		return hint("Register the agent again with auth0 agents register <agent-url>.")
	}

	args := []string{"agents", "register", quoteArg(agent.URL)}
	for _, api := range apis {
		args = append(args, "--api", quoteArg(api))
	}

	return command(r.auth0(args...))
}

// checkAgentApp runs D11 on one agent.
func (r *runner) checkAgentApp(agent coreagents.GrantedAgent, c *collector) {
	name := agentName(agent)
	register := r.registerFix(agent)

	switch agent.MetadataStatus {
	case metadataAvailable:
		c.pass("D11.metadata", specAgentApp, name+": metadata is available")
	case metadataUnknown:
		c.warn("D11.metadata_unknown", specAgentApp, name+": the metadata status could not be read", fix{})
	default:
		c.fail("D11.metadata_status", specAgentApp, fmt.Sprintf("%s: metadata status is %q", name, agent.MetadataStatus),
			fix{command: register.command, hint: "Make the metadata document reachable and valid, then register the agent again."})
	}

	switch {
	case agent.RecordURL == "":
		c.warn("D11.record_missing", specAgentApp, name+": no agent record is linked to the app", register)
	case agent.RecordURL != agent.URL:
		c.warn("D11.record_mismatch", specAgentApp, fmt.Sprintf("%s: the agent record has external_agent_id %q", name, agent.RecordURL), register)
	default:
		c.pass("D11.record", specAgentApp, name+": the agent record is linked with external_agent_id "+agent.RecordURL)
	}

	// DPoP is opt-in, so it is reported but never a problem.
	switch {
	case !agent.DPoPRequired.Known:
	case agent.DPoPRequired.Value:
		c.pass("D11.dpop_required", specAgentApp, name+": DPoP required")
	default:
		c.pass("D11.dpop_optional", specAgentApp, name+": DPoP not required (opt-in)")
	}
}

// checkAgentKeys runs D12 on one agent.
func (r *runner) checkAgentKeys(ctx context.Context, t *Tenant, agent coreagents.GrantedAgent, c *collector) {
	name := agentName(agent)
	register := r.registerFix(agent)

	detail, err := t.Agents.Show(ctx, cmp.Or(agent.URL, agent.AgentID))
	if err != nil {
		r.unreadable(c, "D12", specAgentApp, "the metadata of "+name, scopeAgentKeys, err)
		return
	}

	switch check := detail.MetadataCheck; {
	case !check.Valid.Known:
	case check.Valid.Value:
		c.pass("D12.metadata_valid", specAgentApp, name+": the metadata document is valid")
	default:
		c.fail("D12.metadata_invalid", specAgentApp, fmt.Sprintf("%s: the metadata document is invalid: %s", name, strings.Join(check.Violations, "; ")),
			fix{command: register.command, hint: "Fix the document, then register the agent again."})
	}

	// The preview lists the keys Auth0 stores, so they are compared with the stored
	// ones. A document without a key Auth0 can store is a warning, as the agent has
	// no key to authenticate with.
	check := detail.MetadataCheck

	switch keys := detail.KeysCurrent; {
	case !keys.Known:
	case !keys.Value:
		c.fail("D12.keys_stale", specAgentKeys, name+": the document has other keys than the ones Auth0 stored", register)
	case check.UsableKeys == 0:
		c.warn("D12.keys_none_stored", specAgentKeys, name+": Auth0 stored no key for this agent, because no key of the document matches its token_endpoint_auth_signing_alg",
			hint("Publish a key whose alg matches token_endpoint_auth_signing_alg (RS256 when it is not set), then register the agent again."))
	default:
		c.pass("D12.keys_current", specAgentKeys, name+": the keys Auth0 stored match the document")
	}

	for _, note := range detail.Notes {
		c.warn("D12.note", specAgentApp, fmt.Sprintf("cannot check %s: %s", name, strings.TrimSuffix(note, ".")), fix{})
	}
}

// checkAgentGrants runs D13 on one agent. An agent that is granted some of the
// Poppy APIs only is a valid choice, so it is a warning; an agent that can use
// none of them, or has no grant, cannot act for a person on any Poppy API.
func (r *runner) checkAgentGrants(agent coreagents.GrantedAgent, apis []coreagents.API, c *collector) {
	name := agentName(agent)

	if !agent.APIs.Known {
		c.warn("D13.grants_unknown", "", fmt.Sprintf("cannot check %s: the grants of the agent could not be read", name), fix{})
		return
	}

	var missing []string
	for _, api := range apis {
		// Only a grant for users lets the agent act for a person, so a grant of the
		// app itself, such as a machine-to-machine one, does not count.
		if !slices.ContainsFunc(agent.APIs.Value, func(grant coreagents.Grant) bool {
			return grant.Audience == api.Identifier && grant.IsUserGrant()
		}) {
			missing = append(missing, api.Identifier)
		}
	}

	switch {
	case !agent.MayHaveGrants():
		c.warn("D13.no_grants", "", name+" has no grant to a Poppy API", r.registerFix(agent, missing...))
	case len(missing) == 0:
		c.pass("D13.granted", "", name+" is granted every Poppy API")
	case len(missing) == len(apis):
		for _, api := range missing {
			c.fail("D13.not_granted", "", fmt.Sprintf("%s is not granted %s", name, api), r.registerFix(agent, api))
		}
	default:
		c.warn("D13.partially_granted", "",
			fmt.Sprintf("%s is granted %d of %d Poppy APIs (missing: %s)", name, len(apis)-len(missing), len(apis), strings.Join(missing, ", ")),
			r.registerFix(agent, missing...))
	}
}
