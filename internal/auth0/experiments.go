//go:generate go tool mockgen -source=experiments.go -destination=mock/experiments_mock.go -package=mock

package auth0

import (
	"context"

	management "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// ExperimentListPage aliases the checkpoint-paginated experiments list response.
// The alias keeps the interface return type a single identifier so mockgen's
// source parser can handle it (it cannot parse the multi-type-parameter generic
// inline).
type ExperimentListPage = core.Page[*string, *management.ExperimentListItem, *management.ListExperimentsResponseContent]

// ExperimentsAPI describes the interface for experiment operations.
type ExperimentsAPI interface {
	List(ctx context.Context, request *management.ListExperimentsRequestParameters, opts ...option.RequestOption) (*ExperimentListPage, error)
	Create(ctx context.Context, request *management.CreateExperimentRequestContent, opts ...option.RequestOption) (*management.CreateExperimentResponseContent, error)
	Get(ctx context.Context, id string, opts ...option.RequestOption) (*management.GetExperimentResponseContent, error)
	Update(ctx context.Context, id string, request *management.UpdateExperimentRequestParameters, opts ...option.RequestOption) (*management.UpdateExperimentResponseContent, error)
	Delete(ctx context.Context, id string, opts ...option.RequestOption) error
	AdvanceRamp(ctx context.Context, id string, request *management.AdvanceRampRequestContent, opts ...option.RequestOption) (*management.AdvanceRampResponseContent, error)
	Validate(ctx context.Context, id string, opts ...option.RequestOption) (*management.ValidateExperimentResponseContent, error)
}
