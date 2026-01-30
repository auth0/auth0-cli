package cli

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
)

// Precompiled regex patterns for parsing Terraform config.
var (
	sensitiveNullPattern = regexp.MustCompile(`^(\s*)(\S+)(\s*=\s*)null(\s*#\s*sensitive\s*)$`)
	hasSensitiveRegex    = regexp.MustCompile(`=\s*null\s*#\s*sensitive`)
)

// hasSensitiveNullValues checks if the generated Terraform config contains
// any sensitive fields set to null. This is useful to determine if a
// terraform plan failure might be due to missing sensitive values.
func hasSensitiveNullValues(outputDIR string) bool {
	generatedFilePath := path.Join(outputDIR, generatedTFFileName)

	content, err := os.ReadFile(generatedFilePath)
	if err != nil {
		return false
	}

	return hasSensitiveRegex.Match(content)
}

// processSensitiveFieldsInConfig scans the generated Terraform config file for sensitive fields
// (marked with `null # sensitive`) and replaces them with empty strings and a TODO comment
// instructing users to provide the actual sensitive values.
func processSensitiveFieldsInConfig(outputDIR string) error {
	generatedFilePath := path.Join(outputDIR, generatedTFFileName)

	content, err := os.ReadFile(generatedFilePath)
	if err != nil {
		return fmt.Errorf("failed to read generated config: %w", err)
	}

	updatedContent := replaceSensitiveNullWithEmptyString(string(content))

	if err := os.WriteFile(generatedFilePath, []byte(updatedContent), 0644); err != nil {
		return fmt.Errorf("failed to update generated config: %w", err)
	}

	return nil
}

// replaceSensitiveNullWithEmptyString replaces `null # sensitive` with empty string
// and adds a TODO comment for the user to provide the actual value.
func replaceSensitiveNullWithEmptyString(content string) string {
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		if matches := sensitiveNullPattern.FindStringSubmatch(line); matches != nil {
			/*
				matches[1] = leading whitespace
				matches[2] = attribute name
				matches[3] = " = "
				matches[4] = "# sensitive" comment
			*/
			lines[i] = fmt.Sprintf("%s%s%s\"\" # TODO: Add sensitive value for '%s'",
				matches[1], matches[2], matches[3], matches[2])
		}
	}

	return strings.Join(lines, "\n")
}
