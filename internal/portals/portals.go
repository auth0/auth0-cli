// Package portals sets up and audits Universal Portals in an Auth0 tenant.
//
// The package holds the rules and the order of the work. It never talks to the Management API
// itself: the tenant is reached through the stores in store.go, which the CLI implements.
package portals
