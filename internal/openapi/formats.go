package openapi

import (
	"errors"
	"net/netip"

	"github.com/getkin/kin-openapi/openapi3"
)

// ipStringFormats validates the IP and CIDR string formats in the Management API
// spec. A format without a validator is ignored by kin-openapi, so every string
// would match every branch of a oneOf that differs only by format (for example
// ipv4 or cidr), and the oneOf would reject each value as matching more than one.
// The rules match what the API accepts: netip parsing, with one IP family per format.
var ipStringFormats = map[string]openapi3.StringFormatValidator{
	"ipv4":      openapi3.NewIPValidator(true),
	"ipv6":      openapi3.NewIPValidator(false),
	"cidr":      newCIDRValidator(true),
	"ipv6_cidr": newCIDRValidator(false),
}

// newCIDRValidator accepts a CIDR block of one IP family. Host bits may be set
// (192.0.2.10/24), because the API accepts them.
func newCIDRValidator(isIPv4 bool) openapi3.StringFormatValidator {
	return openapi3.NewCallbackValidator(func(value string) error {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return errors.New("not a CIDR block")
		}
		if isIPv4 && !prefix.Addr().Is4() {
			return errors.New("not an IPv4 CIDR block (it's IPv6)")
		}
		if !isIPv4 && !prefix.Addr().Is6() {
			return errors.New("not an IPv6 CIDR block (it's IPv4)")
		}
		return nil
	})
}
