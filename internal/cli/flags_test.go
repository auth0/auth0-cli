package cli

import "testing"

func TestFlagsRequiredClone(t *testing.T) {
	orig := &Flag{
		Name: "some-flag",
	}

	clone := orig.Required()
	if !clone.IsRequired {
		t.Fatal("wanted flag to be required")
	}

	if orig.IsRequired {
		t.Fatal("wanted original to be left intact")
	}
}
