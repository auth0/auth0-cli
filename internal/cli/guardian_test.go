package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGuardianLegacyPhoneHint(t *testing.T) {
	t.Run("returns nil unchanged", func(t *testing.T) {
		assert.NoError(t, guardianLegacyPhoneHint(nil))
	})

	t.Run("passes through an unrelated error untouched", func(t *testing.T) {
		original := errors.New("failed to read phone provider: 404 not found")

		got := guardianLegacyPhoneHint(original)

		assert.Equal(t, original, got)
	})

	t.Run("augments the legacy phone-provider error with guidance", func(t *testing.T) {
		original := fmt.Errorf("failed to read phone provider: 403 %s", legacyPhoneProviderErrorCode)

		got := guardianLegacyPhoneHint(original)

		// The original error is preserved (wrapped) so callers keep the code.
		assert.ErrorIs(t, got, original)
		assert.Contains(t, got.Error(), legacyPhoneProviderErrorCode)

		// The hint names the actual mechanism and the recommended path forward.
		assert.Contains(t, got.Error(), "legacy_mfa_phone_provider migration flag")
		assert.Contains(t, got.Error(), "PATCH /api/v2/migrations")
		assert.Contains(t, got.Error(), "unified phone experience")
	})
}

func TestIsEmptyResponseErr(t *testing.T) {
	t.Run("false for nil", func(t *testing.T) {
		assert.False(t, isEmptyResponseErr(nil))
	})

	t.Run("false for an unrelated error", func(t *testing.T) {
		assert.False(t, isEmptyResponseErr(errors.New("403 forbidden")))
	})

	t.Run("true for the go-auth0 empty-body error", func(t *testing.T) {
		// Matches the SDK caller's wording for a response with no body.
		err := fmt.Errorf("expected a *management.GetGuardianFactorPhoneTemplatesResponseContent response, but the server responded with nothing")
		assert.True(t, isEmptyResponseErr(err))
	})
}
