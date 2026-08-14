package display

import (
	"testing"

	"github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
)

func TestFormatActionModules(t *testing.T) {
	t.Run("it returns an empty string when there are no modules", func(t *testing.T) {
		assert.Equal(t, "", formatActionModules(nil))
		assert.Equal(t, "", formatActionModules(&[]management.ActionModules{}))
	})

	t.Run("it formats a module with only an id", func(t *testing.T) {
		modules := &[]management.ActionModules{
			{ModuleID: auth0.String("mod_123")},
		}
		assert.Equal(t, "mod_123", formatActionModules(modules))
	})

	t.Run("it formats a module with name and version", func(t *testing.T) {
		modules := &[]management.ActionModules{
			{
				ModuleID:            auth0.String("mod_123"),
				ModuleName:          auth0.String("my-module"),
				ModuleVersionNumber: auth0.Int(2),
			},
		}
		assert.Equal(t, "my-module (mod_123) v2", formatActionModules(modules))
	})

	t.Run("it joins multiple modules", func(t *testing.T) {
		modules := &[]management.ActionModules{
			{ModuleID: auth0.String("mod_123"), ModuleName: auth0.String("first")},
			{ModuleID: auth0.String("mod_456")},
		}
		assert.Equal(t, "first (mod_123), mod_456", formatActionModules(modules))
	})
}
