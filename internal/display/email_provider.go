package display

import (
	"encoding/json"

	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type emailProviderView struct {
	Provider           string
	Enabled            string
	DefaultFromAddress string
	Credentials        string
	Settings           string

	raw interface{}
}

func (v *emailProviderView) AsTableHeader() []string {
	return []string{"Provider", "Enabled", "DefaultFromAddress", "Settings"}
}

func (v *emailProviderView) AsTableRow() []string {
	return []string{
		v.Provider,
		v.Enabled,
		v.DefaultFromAddress,
		v.Settings,
	}
}

func (v *emailProviderView) KeyValues() [][]string {
	return [][]string{
		{"PROVIDER", v.Provider},
		{"ENABLED", v.Enabled},
		{"DEFAULT FROM ADDRESS", v.DefaultFromAddress},
		{"SETTINGS", v.Settings},
	}
}

func (v *emailProviderView) Object() interface{} {
	return v.raw
}

func (r *Renderer) EmailProviderShow(emailProvider *management.EmailProvider) error {
	r.Heading("email provider")
	view, err := makeEmailProviderView(emailProvider)
	if err != nil {
		return err
	}
	r.Result(view)
	return nil
}

func (r *Renderer) EmailProviderCreate(emailProvider *management.EmailProvider) error {
	r.Heading("email provider created")

	view, err := makeEmailProviderView(emailProvider)
	if err != nil {
		return err
	}
	r.Result(view)
	r.Newline()

	// TODO(cyx): possibly guard this with a --no-hint flag.
	r.Infof("%s To edit the email provider, run `auth0 email provider update`",
		ansi.Faint("Hint:"),
	)

	return nil
}

func (r *Renderer) EmailProviderUpdate(emailProvider *management.EmailProvider) error {
	r.Heading("email provider updated")

	view, err := makeEmailProviderView(emailProvider)
	if err != nil {
		return err
	}
	r.Result(view)

	return nil
}

func makeEmailProviderView(emailProvider *management.EmailProvider) (*emailProviderView, error) {
	credentials, err := formatProviderCredentials(emailProvider.Credentials)
	if err != nil {
		return nil, err
	}

	settings, err := formatProviderSettings(emailProvider.Settings)
	if err != nil {
		return nil, err
	}

	return &emailProviderView{
		Provider:           emailProvider.GetName(),
		Enabled:            boolean(emailProvider.GetEnabled()),
		DefaultFromAddress: emailProvider.GetDefaultFromAddress(),
		Credentials:        credentials,
		Settings:           settings,

		raw: emailProvider,
	}, nil
}

func formatProviderCredentials(credentials interface{}) (string, error) {
	if credentials == nil {
		return "", nil
	}

	raw, err := json.Marshal(credentials)
	if err != nil {
		return "", err
	}

	return string(raw), nil
}

func formatProviderSettings(settings interface{}) (string, error) {
	if settings == nil {
		return "", nil
	}

	raw, err := json.Marshal(settings)
	if err != nil {
		return "", err
	}

	return string(raw), nil
}
