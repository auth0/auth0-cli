package cli

import (
	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptsPartials_mergePartialsPrompt(t *testing.T) {
	a := &management.PartialsPrompt{Segment: "login", FormContentStart: "<div>a</div>"}
	b := &management.PartialsPrompt{Segment: "login", FormContentEnd: "<div>b</div>"}
	expected := &management.PartialsPrompt{Segment: "login", FormContentStart: a.FormContentStart, FormContentEnd: b.FormContentEnd}

	err := mergePartialsPrompts(a, b)
	assert.NoError(t, err)
	assert.Equal(t, a, expected)
}

func TestPromptsPartials_setPartialsValueForKey(t *testing.T) {
	input := "<div>test</div>"
	a := &management.PartialsPrompt{Segment: "login"}
	expected := &management.PartialsPrompt{Segment: a.Segment, FormContentStart: input}

	err := setPartialsPromptsValueForKey(a, "foobar", input)
	assert.Error(t, err)

	err = setPartialsPromptsValueForKey(a, "form-content-start", input)
	assert.NoError(t, err)
	assert.Equal(t, a, expected)
}

func TestPromptsPartials_getPartialsPromptsValueFromKey(t *testing.T) {
	content := "<div>test</div>"
	a := &management.PartialsPrompt{FormContentStart: content}
	expected := content
	got := getPartialsPromptsValueForKey(a, "form-content-start")
	assert.Equal(t, expected, got)
}

func TestPromptsPartials_setPartialsPromptsValueForKey(t *testing.T) {
	content := "<div>test</div>"
	a := &management.PartialsPrompt{}
	expected := content
	err := setPartialsPromptsValueForKey(a, "form-content-start", content)
	assert.NoError(t, err)
	assert.Equal(t, expected, a.FormContentStart)
}
