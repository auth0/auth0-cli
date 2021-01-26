package auth0

// API mimics `management.Management`s general interface, except it uses
// methods since we can't really mock fields.
type API interface {
	Actions() ActionsAPI
	Client() ClientAPI
	Connection() ConnectionAPI
	Log() LogAPI
	Rule() RuleAPI
	ResourceServer() ResourceServerAPI
}
