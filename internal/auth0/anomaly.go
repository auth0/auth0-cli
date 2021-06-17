//go:generate mockgen -source=anomaly.go -destination=anomaly_mock.go -package=auth0

package auth0

import "gopkg.in/auth0.v5/management"

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
