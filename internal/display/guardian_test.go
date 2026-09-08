package display

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskSecret(t *testing.T) {
	t.Run("renders a not-set placeholder for an empty secret", func(t *testing.T) {
		assert.Contains(t, MaskSecret(""), "(not set)")
	})

	t.Run("never reveals a set secret", func(t *testing.T) {
		masked := MaskSecret("super-secret-value")
		assert.NotContains(t, masked, "super-secret-value")
		assert.Contains(t, masked, "(set)")
		assert.Contains(t, masked, "••••••••")
	})
}

func TestOrDash(t *testing.T) {
	assert.Equal(t, "-", orDash(""))
	assert.Equal(t, "value", orDash("value"))
}

func TestEnabledStatus(t *testing.T) {
	assert.Contains(t, enabledStatus(true), "enabled")
	assert.Contains(t, enabledStatus(false), "disabled")
}

func TestGuardianFactorView(t *testing.T) {
	view := &guardianFactorView{Name: "sms", Enabled: "enabled"}

	assert.Equal(t, []string{"Factor", "Status"}, view.AsTableHeader())
	assert.Equal(t, []string{"sms", "enabled"}, view.AsTableRow())
	assert.Equal(t, [][]string{
		{"FACTOR", "sms"},
		{"STATUS", "enabled"},
	}, view.KeyValues())
}

func TestGuardianDetailView(t *testing.T) {
	rows := [][]string{
		{"PROVIDER", "twilio"},
		{"AUTH TOKEN", MaskSecret("token")},
	}
	view := &guardianDetailView{rows: rows, raw: map[string]string{"provider": "twilio"}}

	assert.Equal(t, rows, view.KeyValues())
	assert.Equal(t, []string{"twilio", MaskSecret("token")}, view.AsTableRow())
	assert.Equal(t, map[string]string{"provider": "twilio"}, view.Object())
}
