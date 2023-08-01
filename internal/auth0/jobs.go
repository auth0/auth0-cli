package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type JobsAPI interface {
	VerifyEmail(ctx context.Context, j *management.Job, opts ...management.RequestOption) (err error)
	Read(ctx context.Context, id string, opts ...management.RequestOption) (j *management.Job, err error)
	ExportUsers(ctx context.Context, j *management.Job, opts ...management.RequestOption) (err error)
	ImportUsers(ctx context.Context, j *management.Job, opts ...management.RequestOption) (err error)
}
