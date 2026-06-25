//go:generate mockgen -source=experiments.go -destination=mock/experiments_mock.go -package=mock

package auth0

import (
	"context"

	management "github.com/auth0/go-auth0/v2/management"
	managementcore "github.com/auth0/go-auth0/v2/management/core"
	managementoption "github.com/auth0/go-auth0/v2/management/option"
)

// ExperimentsAPI describes the interface for experiment operations.
type ExperimentsAPI interface {
	List(ctx context.Context, request *management.ListExperimentsRequestParameters, opts ...managementoption.RequestOption) (*managementcore.Page[*string, *management.ExperimentListItem, *management.ListExperimentsResponseContent], error)
	Create(ctx context.Context, request *management.CreateExperimentRequestContent, opts ...managementoption.RequestOption) (*management.CreateExperimentResponseContent, error)
	Get(ctx context.Context, id string, opts ...managementoption.RequestOption) (*management.GetExperimentResponseContent, error)
	Update(ctx context.Context, id string, request *management.UpdateExperimentRequestParameters, opts ...managementoption.RequestOption) (*management.UpdateExperimentResponseContent, error)
	Delete(ctx context.Context, id string, opts ...managementoption.RequestOption) error
	UpdateStatus(ctx context.Context, id string, request *management.UpdateExperimentStatusRequestContent, opts ...managementoption.RequestOption) (*management.UpdateExperimentStatusResponseContent, error)
	Validate(ctx context.Context, id string, opts ...managementoption.RequestOption) (*management.ValidateExperimentResponseContent, error)
}
