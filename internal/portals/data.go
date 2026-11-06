package portals

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

// The forms and the pages of the portals are data: the JSON in data/ holds them as the Management
// API takes them. A few values are only known when setup runs, and stand in them as $NAME.
//
//go:embed data/forms/*.json data/templates/*.json
var dataFiles embed.FS

// FormKey identifies one of the forms that the pages of a portal use.
type FormKey string

const (
	// FormPersonalInfo updates the profile of the user.
	FormPersonalInfo FormKey = "personalInfo"
	// FormCommunicationPreferences updates the communication preferences of the user.
	FormCommunicationPreferences FormKey = "communicationPreferences"
	// FormPrivacyConsent updates the privacy settings of the user.
	FormPrivacyConsent FormKey = "privacyConsent"
)

// formKeys are the forms in the order setup creates them.
var formKeys = []FormKey{FormPersonalInfo, FormCommunicationPreferences, FormPrivacyConsent}

// formNames are the names of the forms in the tenant.
var formNames = map[FormKey]string{
	FormPersonalInfo:             "Profile update (Universal Portals)",
	FormPrivacyConsent:           "Privacy settings (Universal Portals)",
	FormCommunicationPreferences: "Marketing communication preferences (Universal Portals)",
}

var formFiles = map[FormKey]string{
	FormPersonalInfo:             "data/forms/personal_info.json",
	FormCommunicationPreferences: "data/forms/communication_preferences.json",
	FormPrivacyConsent:           "data/forms/privacy_consent.json",
}

// FormName returns the name of a form in the tenant.
func FormName(key FormKey) string {
	return formNames[key]
}

// formDefinition is a form to import, with the document the API takes.
type formDefinition struct {
	key      FormKey
	name     string
	document map[string]any
}

// buildForms returns the forms every portal uses. Each of them writes to the signed-in user through
// the vault connection.
func buildForms(vaultConnectionID string) ([]formDefinition, error) {
	definitions := make([]formDefinition, 0, len(formKeys))

	for _, key := range formKeys {
		raw, err := dataFiles.ReadFile(formFiles[key])
		if err != nil {
			return nil, fmt.Errorf("read the form %q: %w", key, err)
		}

		text, err := substitute(string(raw), map[string]string{"$VAULT_CONNECTION_ID": vaultConnectionID})
		if err != nil {
			return nil, fmt.Errorf("build the form %q: %w", key, err)
		}

		var document map[string]any
		if err := json.Unmarshal([]byte(text), &document); err != nil {
			return nil, fmt.Errorf("read the form %q: %w", key, err)
		}

		definitions = append(definitions, formDefinition{key: key, name: formNames[key], document: document})
	}

	return definitions, nil
}

// formIDs are the IDs of the forms in the tenant.
type formIDs map[FormKey]string

// portalTemplate is the pages of a kind of portal.
type portalTemplate struct {
	DefaultPage string             `json:"default_page"`
	Sidebar     []SidebarComponent `json:"sidebar"`
	Pages       []Page             `json:"pages"`
}

var templateFiles = map[Template]string{
	TemplateConsumer: "data/templates/consumer.json",
	TemplateBusiness: "data/templates/business.json",
}

// templateFor returns the pages of a kind of portal, with the forms it uses.
func templateFor(template Template, forms formIDs) (portalTemplate, error) {
	path, known := templateFiles[template]
	if !known {
		return portalTemplate{}, fmt.Errorf("unknown template %q", template)
	}

	raw, err := dataFiles.ReadFile(path)
	if err != nil {
		return portalTemplate{}, fmt.Errorf("read the %s template: %w", template, err)
	}

	text, err := substitute(string(raw), map[string]string{
		"$FORM_PERSONAL_INFO":             forms[FormPersonalInfo],
		"$FORM_PRIVACY_CONSENT":           forms[FormPrivacyConsent],
		"$FORM_COMMUNICATION_PREFERENCES": forms[FormCommunicationPreferences],
	})
	if err != nil {
		return portalTemplate{}, fmt.Errorf("build the %s template: %w", template, err)
	}

	var built portalTemplate
	if err := json.Unmarshal([]byte(text), &built); err != nil {
		return portalTemplate{}, fmt.Errorf("read the %s template: %w", template, err)
	}

	return built, nil
}

// substitute replaces the placeholders in JSON text with values that are valid inside a JSON string.
func substitute(text string, values map[string]string) (string, error) {
	pairs := make([]string, 0, len(values)*2)

	for placeholder, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil {
			return "", fmt.Errorf("encode the value of %s: %w", placeholder, err)
		}

		// A placeholder stands inside quotes, so the quotes of the encoded value are dropped.
		pairs = append(pairs, placeholder, strings.Trim(string(encoded), `"`))
	}

	return strings.NewReplacer(pairs...).Replace(text), nil
}
