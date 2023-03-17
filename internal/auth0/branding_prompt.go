//go:generate mockgen -source=branding_prompt.go -destination=mock/branding_prompt_mock.go -package=mock

package auth0

import "github.com/auth0/go-auth0/management"

type PromptAPI interface {
	// Read retrieves prompts settings.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Prompts/get_prompts
	Read(opts ...management.RequestOption) (p *management.Prompt, err error)

	// Update prompts settings.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Prompts/patch_prompts
	Update(p *management.Prompt, opts ...management.RequestOption) error

	// CustomText retrieves the custom text for a specific prompt and language.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Prompts/get_custom_text_by_language
	CustomText(p string, l string, opts ...management.RequestOption) (t map[string]interface{}, err error)

	// SetCustomText sets the custom text for a specific prompt. Existing texts will be overwritten.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Prompts/put_custom_text_by_language
	SetCustomText(p string, l string, b map[string]interface{}, opts ...management.RequestOption) (err error)
}
