package auth0

type Quickstart struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Samples []string `json:"samples"`
	Org     string   `json:"org"`
	Repo    string   `json:"repo"`
	Branch  string   `json:"branch,omitempty"`
}
