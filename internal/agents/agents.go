// Package agents implements the agent commands (register, unregister, list and
// show) on top of narrow interfaces over the Management API. It has no
// dependency on Cobra, so every flow can be tested with mocks.
package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"

	"github.com/auth0/auth0-cli/internal/auth0"
)

// unknown is what a value reads as when the call that provides it failed.
const unknown = "unknown"

// clientFields limits the clients the commands read to what they use.
const clientFields = "client_id,name,agent_id,external_client_id,external_metadata_type,external_metadata_status,require_proof_of_possession"

// ClientAPI is the part of the clients API the agent commands use.
type ClientAPI interface {
	List(ctx context.Context, request *managementv3.ListClientsRequestParameters, opts ...option.RequestOption) (*auth0.ClientListPage, error)
	Update(ctx context.Context, id string, request *managementv3.UpdateClientRequestContent, opts ...option.RequestOption) (*managementv3.UpdateClientResponseContent, error)
	Delete(ctx context.Context, id string, opts ...option.RequestOption) error
	PreviewCimdMetadata(ctx context.Context, request *managementv3.PreviewCimdMetadataRequestContent, opts ...option.RequestOption) (*managementv3.PreviewCimdMetadataResponseContent, error)
	RegisterCimdClient(ctx context.Context, request *managementv3.RegisterCimdClientRequestContent, opts ...option.RequestOption) (*managementv3.RegisterCimdClientResponseContent, error)
}

// AgentAPI is the part of the agents API the agent commands use.
type AgentAPI interface {
	List(ctx context.Context, request *managementv3.ListAgentsRequestParameters, opts ...option.RequestOption) (*auth0.AgentListPage, error)
	Create(ctx context.Context, request *managementv3.CreateAgentRequestContent, opts ...option.RequestOption) (*managementv3.AgentResponseContent, error)
	Read(ctx context.Context, id string, opts ...option.RequestOption) (*managementv3.AgentResponseContent, error)
	Delete(ctx context.Context, id string, opts ...option.RequestOption) error
}

// CredentialAPI is the part of the client credentials API the agent commands use.
type CredentialAPI interface {
	List(ctx context.Context, clientID string, opts ...option.RequestOption) ([]*managementv3.ClientCredential, error)
}

// Service runs the agent commands against the Management API.
type Service struct {
	Clients     ClientAPI
	Agents      AgentAPI
	Credentials CredentialAPI
	// Grants and ResourceServers are used by what deals with the access of an
	// agent app to APIs.
	Grants          GrantAPI
	ResourceServers ResourceServerAPI
	// Tenant is the tenant chosen with --tenant, which the commands the service
	// writes for a person to run repeat. It is empty for the default tenant.
	Tenant string
}

// Maybe is a value that may be unknown. An unknown value is encoded as null, so
// it is never omitted from the JSON output and keeps the type of the value. The
// items that hold one list the fields that could not be read in unknown_fields.
type Maybe[T any] struct {
	Value T
	Known bool
}

// MarshalJSON encodes the value, or null when it is not known.
func (m Maybe[T]) MarshalJSON() ([]byte, error) {
	if !m.Known {
		return []byte("null"), nil
	}

	return json.Marshal(m.Value)
}

// Known wraps a value that was read.
func Known[T any](value T) Maybe[T] {
	return Maybe[T]{Value: value, Known: true}
}

// PartialError reports a command that stopped after changing some things.
type PartialError struct {
	// The changes that were made, the one that failed and those not tried, by
	// description.
	Applied   []string
	Failed    string
	Remaining []string
	Err       error
}

// Error lists what was done and what is left.
func (e *PartialError) Error() string {
	var b strings.Builder

	fmt.Fprintf(&b, "stopped partway: %v\n  applied: %s\n  failed: %s", e.Err, strings.Join(e.Applied, ", "), e.Failed)

	if len(e.Remaining) > 0 {
		fmt.Fprintf(&b, "\n  remaining: %s", strings.Join(e.Remaining, ", "))
	}

	return b.String()
}

// Unwrap returns the error that stopped the command.
func (e *PartialError) Unwrap() error { return e.Err }

// Step is one change a command makes. It reads as "<verb> <noun>".
type Step struct {
	Verb string
	Noun string
	Run  func(ctx context.Context) error
}

func (s Step) String() string { return s.Verb + " " + s.Noun }

// RunSteps runs the steps in order and stops at the first failure. A failure
// after the first step is reported as a *PartialError.
func RunSteps(ctx context.Context, steps []Step) error {
	for i, s := range steps {
		if err := s.Run(ctx); err != nil {
			err = fmt.Errorf("failed to %s: %w", s, err)
			if i == 0 {
				return err
			}

			return &PartialError{
				Applied:   stepStrings(steps[:i]),
				Failed:    s.String(),
				Remaining: stepStrings(steps[i+1:]),
				Err:       err,
			}
		}
	}

	return nil
}

// stepStrings reads each step as "<verb> <noun>".
func stepStrings(steps []Step) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.String()
	}

	return out
}

// Collect reads every page of a list response.
func Collect[C comparable, T, R any](ctx context.Context, page *core.Page[C, T, R]) ([]T, error) {
	var items []T

	iter := page.Iterator()
	for iter.Next(ctx) {
		items = append(items, iter.Current())
	}

	return items, iter.Err()
}

// statusOf returns the HTTP status carried by a Management API error, or 0.
func statusOf(err error) int {
	var apiErr *core.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}

	return 0
}

// IsURL reports whether ref is an agent URL rather than an agent ID.
func IsURL(ref string) bool {
	return strings.HasPrefix(ref, "https://")
}
