package cli

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestBrandingTextsShowCmd(t *testing.T) {
	tests := []struct {
		name         string
		assertOutput func(t testing.TB, out string)
		args         []string
	}{
		{
			name: "happy path",
			assertOutput: func(t testing.TB, out string) {
				assert.Equal(t, "{}", out)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			brandingTextsAPI := auth0.NewMockPromptAPI(ctrl)
			brandingTextsAPI.EXPECT().
				CustomText(gomock.Any(), gomock.Any()).
				Return(make(map[string]interface{}), nil)

			stdout := &bytes.Buffer{}
			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: ioutil.Discard,
					ResultWriter:  stdout,
				},
				api: &auth0.API{Prompt: brandingTextsAPI},
			}

			cmd := showBrandingTextCmd(cli)
			cmd.SetArgs(test.args)

			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}

			test.assertOutput(t, stdout.String())
		})
	}
}

func TestBrandingTextsUpdateCmd(t *testing.T) {
	tests := []struct {
		name         string
		assertOutput func(t testing.TB, out string)
		args         []string
	}{
		{
			name: "happy path",
			assertOutput: func(t testing.TB, out string) {
				assert.Equal(t, "{}", out)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			brandingTextsAPI := auth0.NewMockPromptAPI(ctrl)
			brandingTextsAPI.EXPECT().
				SetCustomText(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(make(map[string]interface{}))

			stdout := &bytes.Buffer{}
			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: ioutil.Discard,
					ResultWriter:  stdout,
				},
				api: &auth0.API{Prompt: brandingTextsAPI},
			}

			cmd := updateBrandingTextCmd(cli)
			cmd.SetArgs(test.args)

			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}

			test.assertOutput(t, stdout.String())
		})
	}
}
