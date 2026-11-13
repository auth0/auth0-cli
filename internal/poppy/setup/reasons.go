package setup

// The reasons of an InputError. Each is a stable, machine-readable identifier.
const (
	// ReasonInvalidFlag is a flag value that is malformed or that cannot be
	// combined with another flag.
	ReasonInvalidFlag = "invalid_flag"
	// ReasonNothingToSetUp is a run that turns both parts off.
	ReasonNothingToSetUp = "nothing_to_set_up"
	// ReasonAPINotFound is an API that is not in the tenant, or that no config file
	// entry points at.
	ReasonAPINotFound = "api_not_found"
	// ReasonAmbiguousAPI is a config file entry that could be any of several APIs.
	ReasonAmbiguousAPI = "ambiguous_api"
	// ReasonAmbiguousAPIEntry is an API that several config file entries point at.
	ReasonAmbiguousAPIEntry = "ambiguous_api_entry"
	// ReasonAppNotFound is a web app that is not in the tenant, or that no app of
	// the tenant matches.
	ReasonAppNotFound = "app_not_found"
	// ReasonAmbiguousApp is a web app that could be any of several apps.
	ReasonAmbiguousApp = "ambiguous_app"
	// ReasonAppNotFirstParty is a web app that is not first-party.
	ReasonAppNotFirstParty = "app_not_first_party"
	// ReasonCompanyDomainsFull is a tenant that cannot list one more company domain.
	ReasonCompanyDomainsFull = "company_domains_full"
	// ReasonScopeDescriptionExists is a --scope description for a scope that has one.
	ReasonScopeDescriptionExists = "scope_description_exists"
	// ReasonScopeDescriptionRequired is scopes that have no description.
	ReasonScopeDescriptionRequired = "scope_description_required"
	// ReasonScopeNotFound is a --scope description for a scope that no API has.
	ReasonScopeNotFound = "scope_not_found"
	// ReasonPoppyUnavailable is a tenant that does not have the Poppy beta.
	ReasonPoppyUnavailable = "poppy_unavailable"
)
