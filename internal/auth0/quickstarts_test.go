package auth0

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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
		assert.Equal(t, fmt.Sprintf("unable to find any quickstarts for: %s", "some-unknown-type"), err.Error())
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
		assert.Equal(t, fmt.Sprintf("quickstart not found for %s", "some-non-existent-qs-type"), err.Error())
	})
}
