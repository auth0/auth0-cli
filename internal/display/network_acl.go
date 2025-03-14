package display

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/auth0/go-auth0/management"
)

type networkACLView struct {
	acl *management.NetworkACL
}

func (v *networkACLView) AsTableHeader() []string {
	return []string{"ID", "Description", "Priority", "Active", "Action"}
}

func (v *networkACLView) AsTableRow() []string {
	action := "block"
	if v.acl.Rule != nil && v.acl.Rule.Action != nil {
		switch {
		case v.acl.Rule.Action.Allow != nil && *v.acl.Rule.Action.Allow:
			action = "allow"
		case v.acl.Rule.Action.Log != nil && *v.acl.Rule.Action.Log:
			action = "log"
		case v.acl.Rule.Action.Redirect != nil && *v.acl.Rule.Action.Redirect:
			action = "redirect"
		}
	}

	return []string{
		v.acl.GetID(),
		v.acl.GetDescription(),
		strconv.Itoa(v.acl.GetPriority()),
		fmt.Sprintf("%v", v.acl.GetActive()),
		action,
	}
}

func (v *networkACLView) KeyValues() [][]string {
	keyValues := [][]string{
		{"ID", v.acl.GetID()},
		{"DESCRIPTION", v.acl.GetDescription()},
		{"PRIORITY", strconv.Itoa(v.acl.GetPriority())},
		{"ACTIVE", fmt.Sprintf("%v", v.acl.GetActive())},
	}

	if v.acl.Rule != nil {
		keyValues = append(keyValues, []string{"SCOPE", v.acl.Rule.GetScope()})

		// Add action information
		action := "block"
		if v.acl.Rule.Action != nil {
			switch {
			case v.acl.Rule.Action.Allow != nil && *v.acl.Rule.Action.Allow:
				action = "allow"
			case v.acl.Rule.Action.Log != nil && *v.acl.Rule.Action.Log:
				action = "log"
			case v.acl.Rule.Action.Redirect != nil && *v.acl.Rule.Action.Redirect:
				action = "redirect"
			}
		}
		keyValues = append(keyValues, []string{"ACTION", action})

		// Add redirect URI if present
		if v.acl.Rule.Action != nil && v.acl.Rule.Action.RedirectURI != nil {
			keyValues = append(keyValues, []string{"REDIRECT URI", *v.acl.Rule.Action.RedirectURI})
		}

		// Add match criteria if present
		if v.acl.Rule.Match != nil {
			match := v.acl.Rule.Match

			if match.AnonymousProxy != nil && *match.AnonymousProxy {
				keyValues = append(keyValues, []string{"ANONYMOUS PROXY", "true"})
			}

			if len(match.Asns) > 0 {
				asns := make([]string, len(match.Asns))
				for i, asn := range match.Asns {
					asns[i] = strconv.Itoa(asn)
				}
				keyValues = append(keyValues, []string{"ASNS", strings.Join(asns, ", ")})
			}

			if match.GeoCountryCodes != nil && len(*match.GeoCountryCodes) > 0 {
				keyValues = append(keyValues, []string{"COUNTRY CODES", strings.Join(*match.GeoCountryCodes, ", ")})
			}

			if match.GeoSubdivisionCodes != nil && len(*match.GeoSubdivisionCodes) > 0 {
				keyValues = append(keyValues, []string{"SUBDIVISION CODES", strings.Join(*match.GeoSubdivisionCodes, ", ")})
			}

			if match.IPv4Cidrs != nil && len(*match.IPv4Cidrs) > 0 {
				keyValues = append(keyValues, []string{"IPV4 CIDRS", strings.Join(*match.IPv4Cidrs, ", ")})
			}

			if match.IPv6Cidrs != nil && len(*match.IPv6Cidrs) > 0 {
				keyValues = append(keyValues, []string{"IPV6 CIDRS", strings.Join(*match.IPv6Cidrs, ", ")})
			}

			if match.Ja3Fingerprints != nil && len(*match.Ja3Fingerprints) > 0 {
				keyValues = append(keyValues, []string{"JA3 FINGERPRINTS", strings.Join(*match.Ja3Fingerprints, ", ")})
			}

			if match.Ja4Fingerprints != nil && len(*match.Ja4Fingerprints) > 0 {
				keyValues = append(keyValues, []string{"JA4 FINGERPRINTS", strings.Join(*match.Ja4Fingerprints, ", ")})
			}

			if match.UserAgents != nil && len(*match.UserAgents) > 0 {
				keyValues = append(keyValues, []string{"USER AGENTS", strings.Join(*match.UserAgents, ", ")})
			}
		}

		// Add not_match criteria if present
		if v.acl.Rule.NotMatch != nil {
			notMatch := v.acl.Rule.NotMatch
			keyValues = append(keyValues, []string{"NOT MATCH", "true"})

			if notMatch.AnonymousProxy != nil && *notMatch.AnonymousProxy {
				keyValues = append(keyValues, []string{"NOT ANONYMOUS PROXY", "true"})
			}

			if len(notMatch.Asns) > 0 {
				asns := make([]string, len(notMatch.Asns))
				for i, asn := range notMatch.Asns {
					asns[i] = strconv.Itoa(asn)
				}
				keyValues = append(keyValues, []string{"NOT ASNS", strings.Join(asns, ", ")})
			}

			if notMatch.GeoCountryCodes != nil && len(*notMatch.GeoCountryCodes) > 0 {
				keyValues = append(keyValues, []string{"NOT COUNTRY CODES", strings.Join(*notMatch.GeoCountryCodes, ", ")})
			}

			if notMatch.GeoSubdivisionCodes != nil && len(*notMatch.GeoSubdivisionCodes) > 0 {
				keyValues = append(keyValues, []string{"NOT SUBDIVISION CODES", strings.Join(*notMatch.GeoSubdivisionCodes, ", ")})
			}

			if notMatch.IPv4Cidrs != nil && len(*notMatch.IPv4Cidrs) > 0 {
				keyValues = append(keyValues, []string{"NOT IPV4 CIDRS", strings.Join(*notMatch.IPv4Cidrs, ", ")})
			}

			if notMatch.IPv6Cidrs != nil && len(*notMatch.IPv6Cidrs) > 0 {
				keyValues = append(keyValues, []string{"NOT IPV6 CIDRS", strings.Join(*notMatch.IPv6Cidrs, ", ")})
			}

			if notMatch.Ja3Fingerprints != nil && len(*notMatch.Ja3Fingerprints) > 0 {
				keyValues = append(keyValues, []string{"NOT JA3 FINGERPRINTS", strings.Join(*notMatch.Ja3Fingerprints, ", ")})
			}

			if notMatch.Ja4Fingerprints != nil && len(*notMatch.Ja4Fingerprints) > 0 {
				keyValues = append(keyValues, []string{"NOT JA4 FINGERPRINTS", strings.Join(*notMatch.Ja4Fingerprints, ", ")})
			}

			if notMatch.UserAgents != nil && len(*notMatch.UserAgents) > 0 {
				keyValues = append(keyValues, []string{"NOT USER AGENTS", strings.Join(*notMatch.UserAgents, ", ")})
			}
		}
	}

	return keyValues
}

func (v *networkACLView) Object() interface{} {
	return v.acl
}

// NetworkACLList displays a list of network ACLs.
func (r *Renderer) NetworkACLList(acls []*management.NetworkACL) {
	if len(acls) == 0 {
		r.EmptyState("network ACLs", "To create one, run: auth0 network-acl create")
		return
	}

	views := make([]View, len(acls))
	for i, acl := range acls {
		views[i] = &networkACLView{acl: acl}
	}

	r.Heading("network ACLs")
	r.Results(views)
}

// NetworkACLShow displays a single network ACL.
func (r *Renderer) NetworkACLShow(acl *management.NetworkACL) {
	r.Heading("network ACL")
	r.Result(&networkACLView{acl: acl})
}

// NetworkACLCreate displays the result of creating a network ACL.
func (r *Renderer) NetworkACLCreate(acl *management.NetworkACL) {
	r.Heading("network ACL created")
	r.Result(&networkACLView{acl: acl})
}

// NetworkACLUpdate displays the result of updating a network ACL.
func (r *Renderer) NetworkACLUpdate(acl *management.NetworkACL) {
	r.Heading("network ACL updated")
	r.Result(&networkACLView{acl: acl})
}
