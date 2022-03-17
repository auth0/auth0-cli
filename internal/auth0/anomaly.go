package auth0

import "github.com/auth0/go-auth0/management"

type AnomalyAPI interface {
	// Check if a given IP address is blocked via the multiple user accounts
	// trigger due to multiple failed logins.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Anomaly/get_ips_by_id
	CheckIP(ip string, opts ...management.RequestOption) (isBlocked bool, err error)

	// Unblock an IP address currently blocked by the multiple user accounts
	// trigger due to multiple failed logins.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Anomaly/delete_ips_by_id
	UnblockIP(ip string, opts ...management.RequestOption) (err error)
}
