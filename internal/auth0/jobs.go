package auth0

import "github.com/auth0/go-auth0/management"

type JobsAPI interface {
	VerifyEmail(j *management.Job, opts ...management.RequestOption) (err error)
	Read(id string, opts ...management.RequestOption) (j *management.Job, err error)
	ExportUsers(j *management.Job, opts ...management.RequestOption) (err error)
	ImportUsers(j *management.Job, opts ...management.RequestOption) (err error)
}
