package poppy

import (
	"context"
	"fmt"

	coreagents "github.com/auth0/auth0-cli/internal/agents"
)

// List returns the Poppy agents: the agents whose app has a grant to a Poppy
// API. With all set it also returns the agents that have none. A value that
// cannot be read, such as the grants of one agent, is reported as unknown.
func (s *Service) List(ctx context.Context, all bool) ([]coreagents.GrantedAgent, error) {
	listed, err := s.ListGranted(ctx, s.Audiences)
	if err != nil {
		return nil, err
	}

	agents := []coreagents.GrantedAgent{}
	for _, agent := range listed {
		if all || agent.MayHaveGrants() {
			agents = append(agents, agent)
		}
	}

	return agents, nil
}

// Show returns one Poppy agent, given its URL or agent ID, with a live check of
// its metadata document and of whether its stored keys are current. A check
// that fails is reported in Notes, not as an error.
func (s *Service) Show(ctx context.Context, ref string) (*coreagents.GrantedDetail, error) {
	e, agent, err := s.FindGranted(ctx, ref, s.Audiences)
	if err != nil {
		return nil, err
	}

	if !e.IsAgentApp() || !agent.MayHaveGrants() {
		return nil, fmt.Errorf("no agent with Poppy access matches %s", ref)
	}

	return s.CheckGranted(ctx, agent), nil
}
