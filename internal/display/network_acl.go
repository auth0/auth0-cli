package display

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/auth0/go-auth0/management"
)

type networkACLView struct {
	ID          string
	Description string
	Priority    string
	Active      string
	Action      string
	Rule        string
	raw         interface{}
}

func (v *networkACLView) AsTableHeader() []string {
	return []string{"ID", "Description", "Priority", "Active", "Action", "Rule"}
}

func (v *networkACLView) AsTableRow() []string {
	return []string{
		v.ID,
		v.Description,
		v.Priority,
		v.Active,
		v.Action,
		v.Rule,
	}
}

func (v *networkACLView) KeyValues() [][]string {
	keyValues := [][]string{
		{"ID", v.ID},
		{"DESCRIPTION", v.Description},
		{"PRIORITY", v.Priority},
		{"ACTIVE", v.Active},
		{"ACTION", v.Action},
	}

	// Get the original ACL object with nil check.
	if v.raw == nil {
		return keyValues
	}

	acl, ok := v.raw.(*management.NetworkACL)
	if !ok {
		return keyValues
	}

	if acl.Rule != nil {
		keyValues = append(keyValues, []string{"SCOPE", acl.Rule.GetScope()})

		// Add redirect URI if present.
		if acl.Rule.Action != nil && acl.Rule.Action.RedirectURI != nil {
			keyValues = append(keyValues, []string{"REDIRECT URI", *acl.Rule.Action.RedirectURI})
		}

		// Add match criteria if present.
		if acl.Rule.Match != nil {
			match := acl.Rule.Match

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

		// Add not_match criteria if present.
		if acl.Rule.NotMatch != nil {
			notMatch := acl.Rule.NotMatch
			keyValues = append(keyValues, []string{"NOT MATCH", "true"})

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
	return v.raw
}

func makeNetworkACLView(acl *management.NetworkACL) *networkACLView {
	action := "block"
	if acl.Rule != nil && acl.Rule.Action != nil {
		switch {
		case acl.Rule.Action.Allow != nil && *acl.Rule.Action.Allow:
			action = "allow"
		case acl.Rule.Action.Log != nil && *acl.Rule.Action.Log:
			action = "log"
		case acl.Rule.Action.Redirect != nil && *acl.Rule.Action.Redirect:
			action = "redirect"
		}
	}
	// Create a copy of all the attributes of acl in new variable called rawData.
	id := acl.GetID()
	description := acl.GetDescription()
	priority := acl.GetPriority()
	active := acl.GetActive()

	// Create a new instance of management.NetworkACL with the ID values.
	rawData := management.NetworkACL{
		ID:          &id,
		Description: &description,
		Priority:    &priority,
		Active:      &active,
		Rule:        acl.GetRule(),
	}

	// Marshal the rule to JSON.
	ruleJSON, err := json.Marshal(acl.Rule)
	if err != nil {
		ruleJSON = []byte("") // Fallback to empty string.
	}

	return &networkACLView{
		ID:          id,
		Description: description,
		Priority:    strconv.Itoa(priority),
		Active:      fmt.Sprintf("%v", active),
		Action:      action,
		Rule:        string(ruleJSON),
		raw:         rawData,
	}
}

// NetworkACLList displays a list of network ACLs.
func (r *Renderer) NetworkACLList(acls []*management.NetworkACL) error {
	if len(acls) == 0 {
		r.EmptyState("network ACLs", "To create one, run: auth0 network-acl create")
		return nil
	}

	views := make([]View, len(acls))
	for i, acl := range acls {
		views[i] = makeNetworkACLView(acl)
	}

	r.Heading("network ACLs")
	r.Results(views)
	return nil
}

// NetworkACLShow displays a single network ACL.
func (r *Renderer) NetworkACLShow(acl *management.NetworkACL) error {
	r.Heading("network ACL")
	r.Result(makeNetworkACLView(acl))
	return nil
}

// NetworkACLCreate displays the result of creating a network ACL.
func (r *Renderer) NetworkACLCreate(acl *management.NetworkACL) error {
	r.Heading("network ACL created")
	r.Result(makeNetworkACLView(acl))
	return nil
}

// NetworkACLUpdate displays the result of updating a network ACL.
func (r *Renderer) NetworkACLUpdate(acl *management.NetworkACL) error {
	r.Heading("network ACL updated")
	r.Result(makeNetworkACLView(acl))
	return nil
}
