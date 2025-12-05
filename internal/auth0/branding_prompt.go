//go:generate mockgen -source=branding_prompt.go -destination=mock/branding_prompt_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type PromptAPI interface {
	// Read retrieves prompts settings.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Prompts/get_prompts
	Read(ctx context.Context, opts ...management.RequestOption) (p *management.Prompt, err error)

	// Update prompts settings.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Prompts/patch_prompts
	Update(ctx context.Context, p *management.Prompt, opts ...management.RequestOption) error

	// CustomText retrieves the custom text for a specific prompt and language.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Prompts/get_custom_text_by_language
	CustomText(ctx context.Context, p string, l string, opts ...management.RequestOption) (t map[string]interface{}, err error)

	// SetCustomText sets the custom text for a specific prompt. Existing texts will be overwritten.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Prompts/put_custom_text_by_language
	SetCustomText(ctx context.Context, p string, l string, b map[string]interface{}, opts ...management.RequestOption) (err error)

	// GetPartials retrieves the partials for a specific prompt.
	//
	// See: https://auth0.com/docs/api/management/v2/prompts/get-partials
	GetPartials(ctx context.Context, prompt management.PromptType, opts ...management.RequestOption) (c *management.PromptScreenPartials, err error)

	// SetPartials sets the partials for a specific prompt.
	//
	// See: https://auth0.com/docs/api/management/v2/prompts/put-partials
	SetPartials(ctx context.Context, prompt management.PromptType, c *management.PromptScreenPartials, opts ...management.RequestOption) error

	// ReadRendering retrieves the settings for the ACUL.
	//
	// See: https://auth0.com/docs/api/management/v2/prompts/get-rendering
	ReadRendering(ctx context.Context, prompt management.PromptType, screen management.ScreenName, opts ...management.RequestOption) (c *management.PromptRendering, err error)

	// UpdateRendering updates the settings for the ACUL.
	//
	// See: https://auth0.com/docs/api/management/v2/prompts/patch-rendering
	UpdateRendering(ctx context.Context, prompt management.PromptType, screen management.ScreenName, c *management.PromptRendering, opts ...management.RequestOption) error

	// BulkUpdateRendering updates multiple rendering settings in a single operation.
	//
	// See: https://auth0.com/docs/api/management/v2/prompts/patch-bulk-rendering
	BulkUpdateRendering(ctx context.Context, c *management.PromptRenderingBulkUpdate, opts ...management.RequestOption) error

	// ListRendering retrieves the settings for the ACUL.
	//
	ListRendering(ctx context.Context, opts ...management.RequestOption) (c *management.PromptRenderingList, err error)
}
