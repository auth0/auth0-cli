package actions

import (
	"errors"
	"time"

	"github.com/auth0/auth0-cli/internal/auth0"
	"gopkg.in/auth0.v5/management"
)

// NewSampledExecutionAPI creates a decorated ActionExecutionAPI which
// implements a leaky bucket based on the given interval.
func NewSampledExecutionAPI(api auth0.ActionExecutionAPI, interval time.Duration) auth0.ActionExecutionAPI {
	return &sampledExecutionAPI{
		api:      api,
		interval: interval,
		timer:    time.NewTimer(0),
	}
}

type sampledExecutionAPI struct {
	auth0.ActionExecutionAPI

	api auth0.ActionExecutionAPI

	interval time.Duration
	timer    *time.Timer
}

// errRateLimited is returned whenever the leaky bucket isn't ready to drip.
var errRateLimited = errors.New("actions: rate limited")

// Read checks if the leaky bucket is ready to drip: if not, then an
// errRateLimited is returned.
func (a *sampledExecutionAPI) Read(id string) (*management.ActionExecution, error) {
	select {
	case <-a.timer.C:
		a.timer.Reset(a.interval)
		return a.api.Read(id)
	default:
		return nil, errRateLimited
	}
}
