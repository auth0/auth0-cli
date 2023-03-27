package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
)

func TestBrandingTextsShowCmd(t *testing.T) {
	tests := []struct {
		name               string
		inputPrompt        string
		inputLanguage      string
		returnedCustomText map[string]interface{}
		returnedError      error
		expectedOutput     string
	}{
		{
			name:          "it can correctly output the custom text",
			inputPrompt:   "login",
			inputLanguage: "es",
			returnedCustomText: map[string]interface{}{
				"login": map[string]string{
					"title": "testTitle",
				},
			},
			returnedError: nil,
			expectedOutput: `{
    "login": {
        "title": "testTitle"
    }
}`,
		},
		{
			name:               "it fails to output the custom text due to api error",
			inputPrompt:        "login",
			inputLanguage:      "es",
			returnedCustomText: nil,
			returnedError:      errors.New("api error"),
			expectedOutput:     "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			brandingTextsAPI := mock.NewMockPromptAPI(ctrl)
			brandingTextsAPI.EXPECT().
				CustomText(test.inputPrompt, test.inputLanguage).
				Return(test.returnedCustomText, test.returnedError)

			actualOutput := &bytes.Buffer{}
			cli := &cli{
				renderer: &display.Renderer{
					MessageWriter: io.Discard,
					ResultWriter:  actualOutput,
				},
				api: &auth0.API{Prompt: brandingTextsAPI},
			}

			cmd := showPromptsTextCmd(cli)
			cmd.SetArgs([]string{test.inputPrompt, "--language=" + test.inputLanguage})

			err := cmd.Execute()

			if test.returnedError != nil {
				expectedErrorMessage := fmt.Errorf(
					"unable to fetch custom text for prompt %s and language %s: %w",
					test.inputPrompt,
					test.inputLanguage,
					test.returnedError,
				)
				assert.EqualError(t, err, expectedErrorMessage.Error())
				assert.Equal(t, test.expectedOutput, actualOutput.String())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.expectedOutput, actualOutput.String())
		})
	}
}
