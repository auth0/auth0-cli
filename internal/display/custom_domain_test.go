package display

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCustomDomainView_AsTableHeader(t *testing.T) {
	mockCustomDomainView := customDomainView{}

	assert.Equal(t, []string{"ID", "Domain", "Status"}, mockCustomDomainView.AsTableHeader())
}

func TestCustomDomainView_AsTableRow(t *testing.T) {
	mockCustomDomainView := customDomainView{
		ID:     "custom-domain-id",
		Domain: "example.com",
		Status: "verified",
	}

	assert.Equal(t, []string{"custom-domain-id", "example.com", "verified"}, mockCustomDomainView.AsTableRow())
}
