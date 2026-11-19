// Package poppy picks out what is specific to Poppy in the generic agents
// service: the APIs with the Personal Agent Protocol on. It has no dependency on
// Cobra, so every flow can be tested with mocks.
package poppy

import (
	"context"
	"errors"
	"fmt"

	coreagents "github.com/auth0/auth0-cli/internal/agents"
)

// ErrNoPoppyAPI is returned when the tenant has no API with the Personal Agent
// Protocol on.
var ErrNoPoppyAPI = errors.New("no Poppy API found")

// Service runs the Poppy agent commands on top of the generic agents service.
type Service struct {
	*coreagents.Service
}

// poppyAPIs returns every API that has the Personal Agent Protocol on: a
// resource server with it on.
func (s *Service) poppyAPIs(ctx context.Context) ([]coreagents.API, error) {
	if s.ResourceServers == nil {
		return nil, errors.New("the resource servers API is not set")
	}

	servers, err := s.ListServers(ctx)
	if err != nil {
		return nil, err
	}

	var apis []coreagents.API
	for _, server := range servers {
		if server.GetUseForPersonalAgentProtocol() {
			apis = append(apis, coreagents.NewAPI(server))
		}
	}

	return apis, nil
}

// PoppyAPIs returns the Poppy APIs picked by ID or identifier, or every Poppy
// API when none are picked.
func (s *Service) PoppyAPIs(ctx context.Context, picked []string) ([]coreagents.API, error) {
	all, err := s.poppyAPIs(ctx)
	if err != nil {
		return nil, err
	}

	if len(picked) == 0 {
		if len(all) == 0 {
			return nil, fmt.Errorf("%w. Run: auth0 poppy setup", ErrNoPoppyAPI)
		}

		return all, nil
	}

	apis, err := coreagents.PickAPIs(all, picked)

	var unknown *coreagents.UnknownAPIError
	if errors.As(err, &unknown) {
		return nil, fmt.Errorf("%q is not a Poppy API", unknown.Ref)
	}

	return apis, err
}

// Audiences returns the identifiers of the Poppy APIs, to narrow what the
// generic service reads to them.
func (s *Service) Audiences(ctx context.Context) (map[string]bool, error) {
	apis, err := s.poppyAPIs(ctx)
	if err != nil {
		return nil, err
	}

	audiences := make(map[string]bool, len(apis))
	for _, api := range apis {
		audiences[api.Identifier] = true
	}

	return audiences, nil
}
