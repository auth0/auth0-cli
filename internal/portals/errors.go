package portals

import (
	"errors"
	"fmt"
	"net/http"
)

// APIError is a failed Management API request, whatever client made it.
type APIError struct {
	// Status is the HTTP status code.
	Status int
	// Code is the errorCode the API returned, if any.
	Code string
	// Message is the message the API returned, if any.
	Message string
}

func (e *APIError) Error() string {
	switch {
	case e.Code != "" && e.Message != "":
		return fmt.Sprintf("%d %s: %s", e.Status, e.Code, e.Message)
	case e.Message != "":
		return fmt.Sprintf("%d: %s", e.Status, e.Message)
	default:
		return fmt.Sprintf("%d: %s", e.Status, http.StatusText(e.Status))
	}
}

// StatusOf returns the HTTP status of an API failure in err's chain, or 0.
func StatusOf(err error) int {
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		return apiErr.Status
	}

	return 0
}

// CodeOf returns the errorCode of an API failure in err's chain, or "".
func CodeOf(err error) string {
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		return apiErr.Code
	}

	return ""
}

// IsStatus reports whether err is an API failure with one of the given statuses.
func IsStatus(err error, statuses ...int) bool {
	status := StatusOf(err)
	if status == 0 {
		return false
	}

	for _, want := range statuses {
		if status == want {
			return true
		}
	}

	return false
}
