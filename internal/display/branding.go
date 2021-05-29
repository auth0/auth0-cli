package display

import (
	"gopkg.in/auth0.v5/management"
)

type brandingView struct {
	AccentColor     string
	BackgroundColor string
	LogoURL         string
	FaviconURL      string
	CustomFontURL   string
}

func (v *brandingView) AsTableHeader() []string {
	return []string{}
}

func (v *brandingView) AsTableRow() []string {
	return []string{}
}

func (v *brandingView) KeyValues() [][]string {
	return [][]string{
		{"ACCENT COLOR", v.AccentColor},
		{"BACKGROUND COLOR", v.BackgroundColor},
		{"LOGO URL", v.LogoURL},
		{"FAVICON URL", v.FaviconURL},
		{"CUSTOM FONT URL", v.CustomFontURL},
	}
}

func (r *Renderer) BrandingShow(data *management.Branding) {
	r.Heading("branding")

	branding := &brandingView{
		AccentColor: data.GetColors().GetPrimary(),
		BackgroundColor: data.GetColors().GetPageBackground(),
		LogoURL: data.GetLogoURL(),
		FaviconURL: data.GetFaviconURL(),
		CustomFontURL: data.GetFont().GetURL(),
	}

	r.Result(branding)
}
