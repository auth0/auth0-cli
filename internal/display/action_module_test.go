package display

import (
	"testing"
	"time"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
)

func TestFormatActionModuleDependencies(t *testing.T) {
	t.Run("returns an empty string when there are no dependencies", func(t *testing.T) {
		assert.Equal(t, "", formatActionModuleDependencies(nil))
	})

	t.Run("formats dependencies as name@version", func(t *testing.T) {
		deps := []*managementv3.ActionModuleDependency{
			{Name: auth0.String("lodash"), Version: auth0.String("4.0.0")},
			{Name: auth0.String("axios"), Version: auth0.String("1.2.3")},
		}

		assert.Equal(t, "lodash@4.0.0, axios@1.2.3", formatActionModuleDependencies(deps))
	})
}

func TestFormatActionModuleSecrets(t *testing.T) {
	t.Run("returns an empty string when there are no secrets", func(t *testing.T) {
		assert.Equal(t, "", formatActionModuleSecrets(nil))
	})

	t.Run("lists only the secret names", func(t *testing.T) {
		secrets := []*managementv3.ActionModuleSecret{
			{Name: auth0.String("API_KEY")},
			{Name: auth0.String("DB_PASSWORD")},
		}

		assert.Equal(t, "API_KEY, DB_PASSWORD", formatActionModuleSecrets(secrets))
	})
}

func TestMakeActionModuleView(t *testing.T) {
	updated := time.Now().Add(-time.Hour)
	created := time.Now().Add(-48 * time.Hour)

	module := &managementv3.GetActionModuleResponseContent{
		ID:                      auth0.String("am_1"),
		Name:                    auth0.String("mymodule"),
		Code:                    auth0.String("module.exports = () => {}"),
		ActionsUsingModuleTotal: auth0.Int(2),
		AllChangesPublished:     auth0.Bool(true),
		LatestVersionNumber:     auth0.Int(3),
		CreatedAt:               &created,
		UpdatedAt:               &updated,
		Dependencies: []*managementv3.ActionModuleDependency{
			{Name: auth0.String("lodash"), Version: auth0.String("4.0.0")},
		},
		Secrets: []*managementv3.ActionModuleSecret{
			{Name: auth0.String("API_KEY")},
		},
	}

	view := makeActionModuleView(module)

	assert.Equal(t, "mymodule", view.Name)
	assert.Equal(t, "v3", view.LatestVersion)
	assert.Equal(t, "2", view.ActionsUsing)
	assert.Equal(t, "lodash@4.0.0", view.Dependencies)
	assert.Equal(t, "API_KEY", view.Secrets)
	assert.Equal(t, "module.exports = () => {}", view.Code)
	assert.Equal(t, module, view.Object())

	keyValues := view.KeyValues()
	assert.Equal(t, [][]string{
		{"ID", ansi.Faint("am_1")},
		{"NAME", "mymodule"},
		{"LATEST VERSION", "v3"},
		{"ACTIONS USING", "2"},
		{"PUBLISHED", boolean(true)},
		{"CREATED AT", view.CreatedAt},
		{"UPDATED AT", view.UpdatedAt},
		{"DEPENDENCIES", "lodash@4.0.0"},
		{"SECRETS", "API_KEY"},
	}, keyValues)

	// CODE is rendered as a separate block, not as a key/value row.
	assert.Equal(t, "module.exports = () => {}", view.Code)
}

func TestMakeActionModuleViewNestedLatestVersion(t *testing.T) {
	// The get/create/update responses nest the version under
	// latest_version.version_number and leave the flat field unset.
	module := &managementv3.GetActionModuleResponseContent{
		ID:   auth0.String("am_1"),
		Name: auth0.String("mymodule"),
		LatestVersion: &managementv3.ActionModuleVersionReference{
			VersionNumber: auth0.Int(2),
		},
	}

	view := makeActionModuleView(module)

	assert.Equal(t, "v2", view.LatestVersion)
}

func TestMakeActionModuleViewWithoutPublishedVersion(t *testing.T) {
	module := &managementv3.GetActionModuleResponseContent{
		ID:   auth0.String("am_2"),
		Name: auth0.String("draftonly"),
	}

	view := makeActionModuleView(module)

	assert.Equal(t, "-", view.LatestVersion)
	assert.Equal(t, "0", view.ActionsUsing)
	assert.Equal(t, "", view.Dependencies)
	assert.Equal(t, "", view.Secrets)
}
