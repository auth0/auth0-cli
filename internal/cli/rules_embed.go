package cli

import (
	_ "embed"
)

var (
	//go:embed data/rule-template-empty-rule.js
	ruleTemplateEmptyRule string

	//go:embed data/rule-template-add-email-to-access-token.js
	ruleTemplateAddEmailToAccessToken string

	//go:embed data/rule-template-check-last-password-reset.js
	ruleTemplateCheckLastPasswordReset string

	//go:embed data/rule-template-ip-address-allow-list.js
	ruleTemplateIPAddressAllowList string

	//go:embed data/rule-template-ip-address-deny-list.js
	ruleTemplateIPAddressDenyList string

	//go:embed data/rule-template-simple-domain-allow-list.js
	ruleTemplateSimpleDomainAllowList string
)
