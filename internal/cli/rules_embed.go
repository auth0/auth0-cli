package cli

import (
	_ "embed"
)

var (
	//go:embed data/rule-template-empty-rule.js
	ruleTemplateEmptyRule string
)
