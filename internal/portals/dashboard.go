package portals

import "strings"

// A tenantLocation says where the dashboard of a tenant is.
type tenantLocation struct {
	base   string
	region string
	name   string
}

// locate reads the dashboard location from the domain of a tenant. For example "acme.us.auth0.com"
// is https://manage.auth0.com, region "us" and name "acme". A tenant without a region code is
// treated as "us".
func locate(tenantDomain string) tenantLocation {
	parts := strings.Split(tenantDomain, ".")
	hasRegion := len(parts) >= 4 && len(parts[1]) == 2

	service := parts[1:]
	if hasRegion {
		service = parts[2:]
	}

	region := "us"
	if hasRegion {
		region = parts[1]
	}

	return tenantLocation{base: "https://manage." + strings.Join(service, "."), region: region, name: parts[0]}
}

func dashboardRoot(tenantDomain string) string {
	location := locate(tenantDomain)

	return location.base + "/dashboard/" + location.region + "/" + location.name
}

// formsRoot is where the Forms app lives, under the path of the tenant. Forms and flows are there.
func formsRoot(tenantDomain string) string {
	location := locate(tenantDomain)

	return "https://forms.auth0.com/tenants/" + location.region + "/" + location.name
}

// ApplicationURL is the settings page of an application in the dashboard.
func ApplicationURL(tenantDomain, clientID string) string {
	return dashboardRoot(tenantDomain) + "/applications/" + clientID + "/settings"
}

// PortalEditURL is the page where a portal is edited in the dashboard.
func PortalEditURL(tenantDomain, portalID string) string {
	return dashboardRoot(tenantDomain) + "/universal-portals/" + portalID + "/edit"
}

// FormEditURL is the page where a form is edited.
func FormEditURL(tenantDomain, formID string) string {
	return formsRoot(tenantDomain) + "/forms/" + formID + "/edit"
}

// FlowEditURL is the page where a flow is edited.
func FlowEditURL(tenantDomain, flowID string) string {
	return formsRoot(tenantDomain) + "/flows/" + flowID + "/edit"
}

// VaultAppsURL opens the list of the AUTH0 app type, because vault connections have no page of their own.
func VaultAppsURL(tenantDomain string) string {
	return formsRoot(tenantDomain) + "/vault/apps/AUTH0/edit"
}
