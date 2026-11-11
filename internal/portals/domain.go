package portals

import (
	"context"
	"fmt"
	"slices"
)

// domainChoice is where a portal is served from.
type domainChoice struct {
	// custom is the custom domain the portal is served from. It is nil when the portal is served from
	// the domain of the tenant.
	custom *CustomDomain
	// reason says why no custom domain is used, and what that costs. It is empty when one is used.
	reason string
}

const missingCustomDomainCost = "Custom code does not run on the domain of the tenant, " +
	"and passkeys enrolled there have to be enrolled again if the portal moves to a custom domain later."

// chooseCustomDomain picks the custom domain a portal is served from: the default one when it is ready,
// then the ready primary one, then the only ready one. Several ready domains with no default or primary
// are not guessed.
func chooseCustomDomain(ctx context.Context, store CustomDomainStore, tenantDomain string) (domainChoice, error) {
	custom, err := store.ReadDefaultCustomDomain(ctx)
	if err != nil && !IsStatus(err, 404) {
		return domainChoice{}, fmt.Errorf("read the custom domain: %w", err)
	}

	if err == nil && custom.IsReady() {
		return domainChoice{custom: &custom}, nil
	}

	domains, err := store.ListCustomDomains(ctx)
	if err != nil {
		return domainChoice{}, fmt.Errorf("list the custom domains: %w", err)
	}

	var ready []CustomDomain

	for _, domain := range domains {
		if domain.IsReady() {
			ready = append(ready, domain)
		}
	}

	if index := slices.IndexFunc(ready, func(domain CustomDomain) bool { return domain.Primary }); index >= 0 {
		return domainChoice{custom: &ready[index]}, nil
	}

	switch len(ready) {
	case 0:
		reason := "The tenant has no custom domain, so the portal is served from " + tenantDomain + ". "
		if len(domains) > 0 {
			reason = "No custom domain is ready, so the portal is served from " + tenantDomain + ". "
		}

		return domainChoice{reason: reason + missingCustomDomainCost}, nil
	case 1:
		return domainChoice{custom: &ready[0]}, nil
	default:
		return domainChoice{reason: "Several custom domains are ready and none is the default or the primary, so the portal is served from " +
			tenantDomain + ". Set a default custom domain to serve it from there."}, nil
	}
}
