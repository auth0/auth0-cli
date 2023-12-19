//go:generate mockgen -source=log.go -destination=mock/log_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type LogAPI interface {
	// Read the data related to the log entry identified by id. This returns a
	// single log entry representation as specified in the schema.
	Read(ctx context.Context, id string, opts ...management.RequestOption) (l *management.Log, err error)

	// List all log entries that match the specified search criteria (or lists all
	// log entries if no criteria are used). Set custom search criteria using the
	// `q` parameter, or search from a specific log id ("search from checkpoint").
	//
	// For more information on all possible event types, their respective acronyms
	// and descriptions, Log Data Event Listing.
	List(ctx context.Context, opts ...management.RequestOption) (l []*management.Log, err error)

	// Search is an alias for List.
	Search(ctx context.Context, opts ...management.RequestOption) ([]*management.Log, error)
}
