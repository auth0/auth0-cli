package auth0

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var mockQuickStarts = Quickstarts{
	Quickstart{
		Name:                 "Express",
		AppType:              "webapp",
		URL:                  "/docs/quickstart/webapp/express",
		Logo:                 "https://cdn2.auth0.com/docs/1.13412.0/img/platforms/javascript.svg",
		DownloadLink:         "/docs/package/v2?repo=auth0-express-webapp-sample&branch=master&path=01-Login",
		DownloadInstructions: "<!-- markdownlint-disable MD041 -->\n<p>To run the sample follow these steps:</p>\n<ol></p>",
	},
	Quickstart{
		Name:                 "Flutter",
		AppType:              "native",
		URL:                  "/docs/quickstart/native/flutter",
		Logo:                 "https://cdn2.auth0.com/docs/1.13412.0/img/platforms/flutter.svg",
		DownloadLink:         "/docs/package/v2?repo=auth0-flutter-samples&branch=main&path=sample",
		DownloadInstructions: "<p>To run the sample follow these steps:</p>\n<ol>\n<li>Set the <strong>Allowed Callback URLs</strong></p>",
	},
}

func TestFilterByType(t *testing.T) {
	t.Run("filter quickstarts by known types", func(t *testing.T) {
		res, err := mockQuickStarts.FilterByType("webapp")
		assert.Len(t, res, 1)
		assert.Equal(t, res[0].Name, "Express")
		assert.NoError(t, err)

		res, err = mockQuickStarts.FilterByType("native")
		assert.Len(t, res, 1)
		assert.Equal(t, res[0].Name, "Flutter")
		assert.NoError(t, err)
	})

	t.Run("filter quickstarts by an unknown type", func(t *testing.T) {
		res, err := mockQuickStarts.FilterByType("some-unknown-type")
		assert.Nil(t, res)
		assert.Error(t, err)
		assert.Equal(t, fmt.Sprintf("failed to find any quickstarts for type: %q", "some-unknown-type"), err.Error())
	})
}

func TestStacks(t *testing.T) {
	t.Run("get quickstart stacks from quickstarts list", func(t *testing.T) {
		res := mockQuickStarts.Stacks()
		assert.Equal(t, res, []string{"Express", "Flutter"})

		res = Quickstarts{}.Stacks()
		assert.Len(t, res, 0)
	})
}

func TestFindByStack(t *testing.T) {
	t.Run("find quickstart stack by known app type", func(t *testing.T) {
		res, err := mockQuickStarts.FindByStack("Express")
		assert.NoError(t, err)
		assert.Equal(t, "Express", res.Name)
	})

	t.Run("find quickstart stack by known app type", func(t *testing.T) {
		res, err := mockQuickStarts.FindByStack("some-non-existent-qs-type")
		assert.Error(t, err)
		assert.Empty(t, res)
		assert.Equal(t, fmt.Sprintf("failed to find any quickstarts for stack: %q", "some-non-existent-qs-type"), err.Error())
	})
}

func TestSupportedBuildTools(t *testing.T) {
	got := SupportedBuildTools()
	assert.Equal(t, []string{"composer", "gradle", "maven", "none", "vite", "webpack"}, got)
	assert.True(t, sort.StringsAreSorted(got))
	assert.NotContains(t, got, "cra")

	// Every returned tool is the third segment of at least one config key.
	for _, tool := range got {
		found := false
		for key := range QuickstartConfigs {
			if len(key) > len(tool) && key[len(key)-len(tool)-1:] == ":"+tool {
				found = true
				break
			}
		}
		assert.True(t, found, "build tool %q is not used by any QuickstartConfigs key", tool)
	}
}

func TestFrameworkSupportedBuildTools_WebpackVariants(t *testing.T) {
	assert.Equal(t, []string{"vite", "webpack"}, FrameworkSupportedBuildTools["spa:react"])
	assert.Equal(t, []string{"vite", "webpack"}, FrameworkSupportedBuildTools["spa:vanilla-javascript"])
	assert.Equal(t, []string{"vite", "webpack"}, FrameworkBuildTools["spa:react"], "react has no :none variant")
	assert.True(t, FrameworkBuildToolRequired("spa:react"))
	assert.False(t, FrameworkBuildToolRequired("regular:tanstack-start"))
	assert.Equal(t, []string{"none"}, FrameworkBuildTools["regular:tanstack-start"])
}

func TestJavaMVCQuickstartConfigs(t *testing.T) {
	assert.Equal(t, []string{"gradle", "maven"}, FrameworkBuildTools["regular:java-mvc"])
	for _, key := range []string{"regular:java-mvc:maven", "regular:java-mvc:gradle"} {
		t.Run(key, func(t *testing.T) {
			cfg, ok := QuickstartConfigs[key]
			require.True(t, ok, "missing config %q", key)
			assert.Equal(t, FileOutputStrategy{Path: ".env", Format: "dotenv"}, cfg.Strategy)
			assert.Equal(t, "regular_web", cfg.RequestParams.AppType)
			assert.Equal(t, "/callback", cfg.RequestParams.CallbackPath)
			assert.Empty(t, cfg.RequestParams.WebOrigins)
			keys := make([]string, 0, len(cfg.EnvValues))
			for k := range cfg.EnvValues {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			assert.Equal(t, []string{"AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_DOMAIN"}, keys)
		})
	}
}

func TestNewQuickstartConfigs(t *testing.T) {
	tests := []struct {
		key              string
		wantAppType      string
		wantCallbackPath string
		wantWebOrigins   bool
		wantEnvKeys      []string
	}{
		{"spa:react:webpack", "spa", "", true, []string{"AUTH0_CLIENT_ID", "AUTH0_DOMAIN"}},
		{"spa:vanilla-javascript:webpack", "spa", "", true, []string{"AUTH0_CLIENT_ID", "AUTH0_DOMAIN"}},
		{"regular:tanstack-start:none", "regular_web", "/auth/callback", false,
			[]string{"APP_BASE_URL", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_DOMAIN", "AUTH0_SECRET"}},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			cfg, ok := QuickstartConfigs[tc.key]
			require.True(t, ok, "missing config %q", tc.key)

			assert.Equal(t, FileOutputStrategy{Path: ".env", Format: "dotenv"}, cfg.Strategy)
			assert.Equal(t, "AUTH0_AUDIENCE", cfg.AudienceVar)
			assert.Equal(t, tc.wantAppType, cfg.RequestParams.AppType)
			assert.Equal(t, tc.wantCallbackPath, cfg.RequestParams.CallbackPath)
			assert.Equal(t, DetectionSub, cfg.RequestParams.Name)
			assert.Equal(t, []string{DetectionSub}, cfg.RequestParams.AllowedLogoutURLs)
			if tc.wantWebOrigins {
				assert.Equal(t, []string{DetectionSubAsBase}, cfg.RequestParams.Callbacks, "SPA callback is the bare base URL")
				assert.Equal(t, []string{DetectionSub}, cfg.RequestParams.WebOrigins)
			} else {
				assert.Equal(t, []string{DetectionSub}, cfg.RequestParams.Callbacks)
				assert.Empty(t, cfg.RequestParams.WebOrigins)
			}

			keys := make([]string, 0, len(cfg.EnvValues))
			for k, v := range cfg.EnvValues {
				keys = append(keys, k)
				assert.Equal(t, DetectionSub, v, "env %q must be resolved at setup time", k)
			}
			sort.Strings(keys)
			assert.Equal(t, tc.wantEnvKeys, keys)
		})
	}

	t.Run("webpack configs mirror their vite counterparts' RequestParams", func(t *testing.T) {
		assert.Equal(t, QuickstartConfigs["spa:react:vite"].RequestParams, QuickstartConfigs["spa:react:webpack"].RequestParams)
		assert.Equal(t, QuickstartConfigs["spa:vanilla-javascript:vite"].RequestParams, QuickstartConfigs["spa:vanilla-javascript:webpack"].RequestParams)
	})

	t.Run("out-of-scope configs are absent", func(t *testing.T) {
		for _, key := range []string{"spa:react:cra", "regular:java-mvc:none"} {
			_, ok := QuickstartConfigs[key]
			assert.False(t, ok, "%q must not exist", key)
		}
	})
}
