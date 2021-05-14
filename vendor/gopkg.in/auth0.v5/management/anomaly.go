package management

import (
	"net/http"
)

type AnomalyManager struct {
	*Management
}

func newAnomalyManager(m *Management) *AnomalyManager {
	return &AnomalyManager{m}
}

// Check if a given IP address is blocked via the multiple user accounts
// trigger due to multiple failed logins.
//
// See: https://auth0.com/docs/api/management/v2#!/Anomaly/get_ips_by_id
func (m *AnomalyManager) CheckIP(ip string, opts ...RequestOption) (isBlocked bool, err error) {
	req, err := m.NewRequest("GET", m.URI("anomaly", "blocks", "ips", ip), nil, opts...)
	if err != nil {
		return false, err
	}

	res, err := m.Do(req)
	if err != nil {
		return false, err
	}

	// 200: IP address specified is currently blocked.
	if res.StatusCode == http.StatusOK {
		return true, nil
	}

	// 404: IP address specified is not currently blocked.
	if res.StatusCode == http.StatusNotFound {
		return false, nil
	}

	return false, newError(res.Body)
}

// Unblock an IP address currently blocked by the multiple user accounts
// trigger due to multiple failed logins.
//
// See: https://auth0.com/docs/api/management/v2#!/Anomaly/delete_ips_by_id
func (m *AnomalyManager) UnblockIP(ip string, opts ...RequestOption) (err error) {
	return m.Request("DELETE", m.URI("anomaly", "blocks", "ips", ip), nil, opts...)
}
