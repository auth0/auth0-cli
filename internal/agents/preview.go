package agents

import (
	"context"
	"fmt"
	"slices"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"
)

// keysNote explains which keys of a document Auth0 stores for an agent app.
const keysNote = "Auth0 stores only the keys whose alg matches the document's token_endpoint_auth_signing_alg, which is RS256 when the document does not set it."

// Key is a public key listed in an agent's metadata document.
type Key struct {
	KID string
	Alg string
}

// Preview is what Auth0 reads from an agent's metadata document.
type Preview struct {
	URL          string
	Name         string
	LogoURI      string
	RedirectURLs []string
	// Keys are the keys Auth0 stores for the agent, as the preview lists them.
	Keys     []Key
	Warnings []string
	// ClientID is the agent app already registered for the URL, if any.
	ClientID string
}

// InvalidDocumentError reports an agent metadata document Auth0 rejects.
type InvalidDocumentError struct {
	URL        string
	Violations []string
}

// Error is one line that names the document. The violations are in Violations,
// and Advice lists them for a person.
func (e *InvalidDocumentError) Error() string {
	return fmt.Sprintf("the agent metadata document at %s is not valid", e.URL)
}

// Advice lists the violations, one per line, and explains which keys Auth0 stores
// when they are the cause.
func (e *InvalidDocumentError) Advice() string {
	var b strings.Builder

	for i, violation := range e.Violations {
		if i > 0 {
			b.WriteString("\n")
		}

		b.WriteString(violation)
	}

	if slices.ContainsFunc(e.Violations, aboutKeys) {
		b.WriteString("\n")
		b.WriteString(keysNote)
	}

	return b.String()
}

// Unreachable reports whether the document could not be retrieved, which is the
// case when every violation says that its retrieval failed.
func (e *InvalidDocumentError) Unreachable() bool {
	return len(e.Violations) > 0 && !slices.ContainsFunc(e.Violations, func(violation string) bool {
		return !strings.Contains(strings.ToLower(violation), "retrieval")
	})
}

func aboutKeys(violation string) bool {
	violation = strings.ToLower(violation)

	return strings.Contains(violation, "key") || strings.Contains(violation, "jwk") || strings.Contains(violation, "alg")
}

// Preview asks Auth0 to fetch and check an agent's metadata document without
// creating anything. A document Auth0 rejects returns an *InvalidDocumentError.
func (s *Service) Preview(ctx context.Context, agentURL string) (*Preview, error) {
	resp, err := s.Clients.PreviewCimdMetadata(ctx, &managementv3.PreviewCimdMetadataRequestContent{ExternalClientID: agentURL})
	if err != nil {
		return nil, fmt.Errorf("failed to preview the agent metadata document: %w", err)
	}

	if errs := resp.GetErrors(); len(errs) > 0 {
		return nil, &InvalidDocumentError{URL: agentURL, Violations: errs}
	}

	if !resp.Validation.GetValid() {
		return nil, &InvalidDocumentError{URL: agentURL, Violations: resp.Validation.GetViolations()}
	}

	fields := resp.MappedFields
	preview := &Preview{
		URL:          agentURL,
		Name:         fields.GetName(),
		LogoURI:      fields.GetLogoURI(),
		RedirectURLs: fields.GetCallbacks(),
		Warnings:     resp.Validation.GetWarnings(),
		ClientID:     resp.GetClientID(),
	}

	if fields != nil && fields.ClientAuthenticationMethods != nil && fields.ClientAuthenticationMethods.PrivateKeyJwt != nil {
		for _, credential := range fields.ClientAuthenticationMethods.PrivateKeyJwt.Credentials {
			preview.Keys = append(preview.Keys, Key{KID: credential.Kid, Alg: credential.Alg})
		}
	}

	return preview, nil
}
