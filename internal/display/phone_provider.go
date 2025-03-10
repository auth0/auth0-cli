package display

import (
	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type phoneProviderView struct {
	ID            string
	Provider      string
	Disabled      string
	Credentials   string
	Configuration string

	raw interface{}
}

func (v *phoneProviderView) Object() interface{} {
	return v.raw
}

func (v *phoneProviderView) AsTableHeader() []string {
	return []string{"ID", "Provider", "Disabled", "Configuration"}
}

func (v *phoneProviderView) AsTableRow() []string {
	return []string{
		v.ID,
		v.Provider,
		v.Disabled,
		v.Configuration,
	}
}

func (v *phoneProviderView) KeyValues() [][]string {
	return [][]string{
		{"ID", v.ID},
		{"PROVIDER", v.Provider},
		{"DISABLED", v.Disabled},
		{"CONFIGURATION", v.Configuration},
	}
}

func (r *Renderer) PhoneProviderList(phoneProviders []*management.BrandingPhoneProvider) error {
	resource := "phone providers"

	r.Heading(resource)

	if len(phoneProviders) == 0 {
		r.EmptyState(resource, "Use 'auth0 phone provider create' to add one")
		return nil
	}

	var res []View
	for _, p := range phoneProviders {
		view, err := makePhoneProviderView(p)
		if err != nil {
			return err
		}

		res = append(res, view)
	}

	r.Results(res)
	return nil
}

func (r *Renderer) PhoneProviderShow(phoneProvider *management.BrandingPhoneProvider) error {
	r.Heading("phone provider")
	view, err := makePhoneProviderView(phoneProvider)
	if err != nil {
		return err
	}
	r.Result(view)
	return nil
}

func (r *Renderer) PhoneProviderCreate(phoneProvider *management.BrandingPhoneProvider) error {
	r.Heading("phone provider created")

	view, err := makePhoneProviderView(phoneProvider)
	if err != nil {
		return err
	}
	r.Result(view)
	r.Newline()

	// TODO(cyx): possibly guard this with a --no-hint flag.
	r.Infof("%s To edit the phone provider, run `auth0 phone provider update`",
		ansi.Faint("Hint:"),
	)

	return nil
}

func (r *Renderer) PhoneProviderUpdate(phoneProvider *management.BrandingPhoneProvider) error {
	r.Heading("phone provider updated")

	view, err := makePhoneProviderView(phoneProvider)
	if err != nil {
		return err
	}
	r.Result(view)

	return nil
}

func makePhoneProviderView(phoneProvider *management.BrandingPhoneProvider) (*phoneProviderView, error) {
	credentials, err := toJSONString(phoneProvider.Credentials)
	if err != nil {
		return nil, err
	}

	configuration, err := toJSONString(phoneProvider.Configuration)
	if err != nil {
		return nil, err
	}

	return &phoneProviderView{
		ID:            phoneProvider.GetID(),
		Provider:      phoneProvider.GetName(),
		Disabled:      boolean(phoneProvider.GetDisabled()),
		Credentials:   credentials,
		Configuration: configuration,

		raw: phoneProvider,
	}, nil
}
