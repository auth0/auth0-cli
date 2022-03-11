package auth0

type ImportChanges struct {
	Resource string
	Creates  int
	Updates  int
	Deletes  int
}
