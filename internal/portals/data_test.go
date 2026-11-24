package portals_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
)

var testFormIDs = portals.FormIDs{
	portals.FormPersonalInfo:             "ap_personal",
	portals.FormPrivacyConsent:           "ap_privacy",
	portals.FormCommunicationPreferences: "ap_comms",
}

func TestForms(t *testing.T) {
	t.Parallel()

	documents, err := portals.BuildFormDocuments("ac_test_vault")
	require.NoError(t, err)

	keys := []portals.FormKey{portals.FormPersonalInfo, portals.FormCommunicationPreferences, portals.FormPrivacyConsent}

	t.Run("there are three forms, with the names they have in the tenant", func(t *testing.T) {
		t.Parallel()

		assert.Len(t, documents, 3)

		for _, key := range keys {
			form, _ := documents[key]["form"].(map[string]any)
			assert.Equal(t, portals.FormName(key), form["name"], string(key))
		}
	})

	t.Run("every form routes its user update through the vault connection", func(t *testing.T) {
		t.Parallel()

		for _, key := range keys {
			action := firstAction(t, documents[key])

			assert.Equal(t, "UPDATE_USER", action["action"], string(key))
			assert.Equal(t, "#CONN-1#", action["params"].(map[string]any)["connection_id"], string(key))
			assert.Equal(t, map[string]any{"#CONN-1#": map[string]any{"id": "ac_test_vault"}}, documents[key]["connections"], string(key))
		}
	})

	t.Run("each form writes its answers to the metadata it is meant to update", func(t *testing.T) {
		t.Parallel()

		expected := map[portals.FormKey]string{
			portals.FormPersonalInfo:             "user_metadata",
			portals.FormCommunicationPreferences: "app_metadata",
			portals.FormPrivacyConsent:           "app_metadata",
		}

		for _, key := range keys {
			changes := firstAction(t, documents[key])["params"].(map[string]any)["changes"].(map[string]any)

			assert.Contains(t, changes, expected[key], string(key))
		}
	})

	t.Run("every form has a step that links to its flow", func(t *testing.T) {
		t.Parallel()

		for _, key := range keys {
			nodes := documents[key]["form"].(map[string]any)["nodes"].([]any)

			var stepNext, flowID string

			for _, node := range nodes {
				entry := node.(map[string]any)

				switch entry["type"] {
				case "STEP":
					stepNext, _ = entry["config"].(map[string]any)["next_node"].(string)
				case "FLOW":
					flowID, _ = entry["id"].(string)
				}
			}

			assert.NotEmpty(t, flowID, "%s has no flow node", key)
			assert.Equal(t, flowID, stepNext, "%s: the step does not lead to the flow", key)
		}
	})

	t.Run("the vault ID is put in as a value, whatever characters it has", func(t *testing.T) {
		t.Parallel()

		built, err := portals.BuildFormDocuments(`ac_"quoted"\id`)
		require.NoError(t, err)

		assert.Equal(t, `ac_"quoted"\id`, built[portals.FormPersonalInfo]["connections"].(map[string]any)["#CONN-1#"].(map[string]any)["id"])
	})
}

func firstAction(t *testing.T, document map[string]any) map[string]any {
	t.Helper()

	flows, ok := document["flows"].(map[string]any)
	require.True(t, ok)

	flow, ok := flows["#FLOW-1#"].(map[string]any)
	require.True(t, ok)

	actions, ok := flow["actions"].([]any)
	require.True(t, ok)

	return actions[0].(map[string]any)
}

// The limits the API puts on a portal.
const (
	maxTitleLength      = 150
	maxSectionChildren  = 20
	sectionComponent    = "page:component:auth0:structure:section"
	formComponent       = "page:component:auth0:form"
	internalLinkV2      = "sidebar:component:auth0:internal_link:v2"
	organizationAPIPart = "my_organization"
)

func TestTemplates(t *testing.T) {
	t.Parallel()

	for _, kind := range []portals.Template{portals.TemplateConsumer} {
		template, err := portals.TemplateFor(kind, testFormIDs)
		require.NoError(t, err)

		t.Run(string(kind)+": every sidebar link points to an existing page", func(t *testing.T) {
			t.Parallel()

			pages := pageSlugs(template)
			for _, item := range template.Sidebar {
				assert.Equal(t, internalLinkV2, item.Type)
				assert.Contains(t, pages, item.Config["page_slug"], "link to a page that does not exist")
			}
		})

		t.Run(string(kind)+": every page is in the sidebar", func(t *testing.T) {
			t.Parallel()

			var linked []string
			for _, item := range template.Sidebar {
				linked = append(linked, item.Config["page_slug"].(string))
			}

			for _, page := range template.Pages {
				assert.Contains(t, linked, page.Slug, "page not in the navigation")
			}
		})

		t.Run(string(kind)+": the default page is one of the pages", func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, pageSlugs(template), template.DefaultPage)
		})

		t.Run(string(kind)+": every slug and icon matches what the API accepts", func(t *testing.T) {
			t.Parallel()

			for _, page := range template.Pages {
				assert.Regexp(t, `^[a-z][a-z0-9-]{0,49}$`, page.Slug)
			}

			for _, item := range template.Sidebar {
				assert.Regexp(t, `^[a-z0-9-]{1,50}$`, item.Config["icon"])
			}
		})

		t.Run(string(kind)+": every title fits the limits", func(t *testing.T) {
			t.Parallel()

			for _, page := range template.Pages {
				assert.NotEmpty(t, page.Title)
				assert.LessOrEqual(t, len(page.Title), maxTitleLength)
			}
		})

		t.Run(string(kind)+": every component is a section, and sections hold only leaf components", func(t *testing.T) {
			t.Parallel()

			for _, page := range template.Pages {
				for _, component := range page.Components {
					assert.Equal(t, sectionComponent, component.Type, "a component outside a section")

					children, _ := component.Config["children"].([]any)
					assert.LessOrEqual(t, len(children), maxSectionChildren)

					for _, child := range children {
						assert.NotEqual(t, sectionComponent, child.(map[string]any)["type"], "sections cannot be nested")
					}
				}
			}
		})

		t.Run(string(kind)+": every form used is one of the forms that were supplied", func(t *testing.T) {
			t.Parallel()

			known := []string{"ap_personal", "ap_privacy", "ap_comms"}
			for _, formID := range formIDsUsed(template.Pages) {
				assert.Contains(t, known, formID)
			}
		})

		t.Run(string(kind)+": no placeholder is left", func(t *testing.T) {
			t.Parallel()

			encoded, err := json.Marshal(template)
			require.NoError(t, err)
			assert.NotContains(t, string(encoded), "$FORM_")
		})
	}

	t.Run("consumer: uses only the My Account API and no organization components", func(t *testing.T) {
		t.Parallel()

		template, err := portals.TemplateFor(portals.TemplateConsumer, testFormIDs)
		require.NoError(t, err)

		encoded, err := json.Marshal(template)
		require.NoError(t, err)
		assert.False(t, strings.Contains(string(encoded), organizationAPIPart), "an organization component in the consumer template")

		assert.Equal(t, "profile", template.DefaultPage)
		assert.Equal(t, []string{"profile", "security", "legal-privacy"}, pageSlugs(template))
	})
}

func pageSlugs(template portals.PortalTemplate) []string {
	slugs := make([]string, 0, len(template.Pages))
	for _, page := range template.Pages {
		slugs = append(slugs, page.Slug)
	}

	return slugs
}

// formIDsUsed returns the IDs of the forms the pages use.
func formIDsUsed(pages []portals.Page) []string {
	var ids []string

	for _, page := range pages {
		for _, component := range page.Components {
			children, _ := component.Config["children"].([]any)

			for _, child := range children {
				entry := child.(map[string]any)
				if entry["type"] == formComponent {
					ids = append(ids, entry["config"].(map[string]any)["form_id"].(string))
				}
			}
		}
	}

	return ids
}

func TestTemplateFor_UnknownTemplate(t *testing.T) {
	t.Parallel()

	_, err := portals.TemplateFor("enterprise", testFormIDs)

	assert.EqualError(t, err, `unknown template "enterprise"`)
}
