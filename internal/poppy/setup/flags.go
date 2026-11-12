package setup

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

// Flags holds the flag values of the setup and config commands, as given.
type Flags struct {
	CompanyDomain string
	APIs          []string
	APIURLs       []string
	App           string
	Scopes        []string
	Hints         []string
	Name          string
	LoginURI      string
	Out           string
	ForceConfig   bool
	// WebUnscoped is the web.unscoped flag. Nil means it was not passed.
	WebUnscoped *bool
	// NoAPI and NoWeb are the --no-api and --no-web flags.
	NoAPI bool
	NoWeb bool
}

// Inputs holds the validated values of the flags and of the answers to the
// questions.
type Inputs struct {
	// CompanyDomain is a bare lower-case host.
	CompanyDomain string
	// APIs are the requested APIs, by ID or identifier.
	APIs []string
	// APIURLs holds the OpenAPI document URLs by API identifier.
	APIURLs map[string]string
	// App is the client ID or name of the web app.
	App string
	// Scopes holds the scope descriptions given by name.
	Scopes      map[string]string
	Hints       map[string]config.Hints
	Name        string
	LoginURI    string
	Out         string
	ForceConfig bool
	// WebUnscoped is the owner's answer for web.unscoped, from the flag or from
	// the question. Nil means it was not given, and the config file's value is
	// kept.
	WebUnscoped *bool
	// NoAPI turns the API part off: no API is read, planned or changed, and the
	// config file gets auth.disabled. It comes from --no-api, from the question, or
	// from a config file that has auth.disabled.
	NoAPI bool
	// NoWeb turns the web part (the browser sign-in) off: the web app is neither
	// read nor changed, and the config file gets web.disabled. It comes from
	// --no-web, from the question, or from a config file that has web.disabled.
	NoWeb bool

	// InFileAPI and inFileWeb tell that the existing config file already answers
	// the question of a part with yes: it lists APIs, or the browser join address.
	// UseConfigFile sets them.
	inFileAPI bool
	inFileWeb bool
}

// Parse validates the flag values. Missing required values are not an error
// here: see RequiredQuestions.
func (f Flags) Parse() (Inputs, error) {
	in := Inputs{
		APIs:        slices.Compact(slices.DeleteFunc(slices.Clone(f.APIs), func(s string) bool { return strings.TrimSpace(s) == "" })),
		App:         strings.TrimSpace(f.App),
		Name:        f.Name,
		Out:         f.Out,
		ForceConfig: f.ForceConfig,
		NoAPI:       f.NoAPI,
		NoWeb:       f.NoWeb,
	}

	if f.NoAPI && f.NoWeb {
		err := invalidFlag("--no-api", "it cannot be used with --no-web: there would be nothing to set up")
		err.Fix = "Drop --no-api or --no-web."

		return Inputs{}, err
	}

	if f.WebUnscoped != nil {
		unscoped := *f.WebUnscoped
		in.WebUnscoped = &unscoped
	}

	var err error

	if f.CompanyDomain != "" {
		if in.CompanyDomain, err = parseDomain(f.CompanyDomain); err != nil {
			return Inputs{}, err
		}
	}

	if f.LoginURI != "" {
		if err = checkLoginURI(f.LoginURI); err != nil {
			return Inputs{}, err
		}
		in.LoginURI = f.LoginURI
	}

	if in.APIURLs, err = parsePairs("--api-url", f.APIURLs, checkAPIURL); err != nil {
		return Inputs{}, err
	}

	if in.Scopes, err = parsePairs("--scope", f.Scopes, checkDescription); err != nil {
		return Inputs{}, err
	}

	if in.Hints, err = parseHints(f.Hints); err != nil {
		return Inputs{}, err
	}

	if err = in.checkParts(); err != nil {
		return Inputs{}, err
	}

	return in, nil
}

// givenFlag is a flag of one part of Poppy and whether the inputs got it.
type givenFlag struct {
	name string
	set  bool
}

// apiFlags are the flags that only mean something when the API part is on.
func (in Inputs) apiFlags() []givenFlag {
	return []givenFlag{
		{"--api", len(in.APIs) > 0}, {"--api-url", len(in.APIURLs) > 0}, {"--scope", len(in.Scopes) > 0},
		{"--hint", len(in.Hints) > 0},
	}
}

// webFlags are the flags that only mean something when the web part is on.
func (in Inputs) webFlags() []givenFlag {
	return []givenFlag{
		{"--app", in.App != ""}, {"--login-uri", in.LoginURI != ""}, {"--web-unscoped", in.WebUnscoped != nil},
	}
}

// firstGiven returns the first flag of the list that was given.
func firstGiven(flags []givenFlag) (string, bool) {
	for _, flag := range flags {
		if flag.set {
			return flag.name, true
		}
	}

	return "", false
}

// checkParts rejects a part that is turned off together with a flag of that part.
func (in Inputs) checkParts() error {
	for _, part := range []struct {
		off   bool
		flag  string
		given []givenFlag
	}{{in.NoAPI, "--no-api", in.apiFlags()}, {in.NoWeb, "--no-web", in.webFlags()}} {
		if !part.off {
			continue
		}

		if given, found := firstGiven(part.given); found {
			err := invalidFlag(part.flag, "it turns that part off, so it cannot be used with %s", given)
			err.Fix = fmt.Sprintf("Drop %s, or drop %s.", part.flag, given)

			return err
		}
	}

	return nil
}

// NothingToSetUp is the error for a run that turns both parts off.
func NothingToSetUp() *InputError {
	return &InputError{
		Reason:  ReasonNothingToSetUp,
		Message: "The API part and the web part are both off, so there is nothing to set up.",
		Fix:     "Keep one of them on: pass --api or --app, or answer yes to its question.",
	}
}

// UseConfigFile turns a part off when the existing config file says so
// (auth.disabled for the API part, web.disabled for the web part) and no flag asks
// for the part: the file already answers its question. A file that lists APIs, or
// the browser join address, answers yes to the question of that part, so that
// AsksAPI and AsksWeb stay false for it. A flag wins over the file.
// It returns NothingToSetUp when that leaves no part on.
func (in *Inputs) UseConfigFile(file *config.Config) error {
	if file == nil {
		return nil
	}

	if _, asked := firstGiven(in.apiFlags()); file.Auth.Disabled && !asked {
		in.NoAPI = true
	}

	if _, asked := firstGiven(in.webFlags()); file.Web.Disabled && !asked {
		in.NoWeb = true
	}

	in.inFileAPI = !in.NoAPI && len(file.APIs) > 0
	in.inFileWeb = !in.NoWeb && file.Web.BrowserSessionEndpoint != ""

	if in.NoAPI && in.NoWeb {
		return NothingToSetUp()
	}

	return nil
}

// AsksAPI tells whether the question whether agents use an API is still open: the
// part is not off, no flag of the part was given, and the config file does not
// already list APIs.
func (in Inputs) AsksAPI() bool {
	_, given := firstGiven(in.apiFlags())

	return !in.NoAPI && !given && !in.inFileAPI
}

// AsksWeb tells whether the question whether agents browse the website signed in
// is still open: the part is not off, no flag of the part was given, and the
// config file does not already have the browser join address.
func (in Inputs) AsksWeb() bool {
	_, given := firstGiven(in.webFlags())

	return !in.NoWeb && !given && !in.inFileWeb
}

// parseDomain checks a company domain and lower-cases it. Only a bare host is
// accepted.
func parseDomain(s string) (string, error) {
	if strings.Contains(s, "://") {
		return "", invalidFlag("--company-domain", "%q must be a host name such as shop.example, without a scheme", s)
	}

	domain, err := config.NormalizeDomain(s)
	if err != nil {
		return "", invalidFlag("--company-domain", "%s", err)
	}

	return domain, nil
}

// parsePairs reads values of the form key=value into a map, checking each value.
func parsePairs(flag string, values []string, check func(flag, value string) (string, error)) (map[string]string, error) {
	pairs := make(map[string]string, len(values))

	for _, value := range values {
		key, v, found := strings.Cut(value, "=")
		key = strings.TrimSpace(key)

		if !found || key == "" {
			return nil, invalidFlag(flag, "%q must be written <name>=<value>, where the first = separates the name from the value", value)
		}

		v, err := check(flag, v)
		if err != nil {
			return nil, err
		}

		pairs[key] = v
	}

	return pairs, nil
}

func checkDescription(flag, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", invalidFlag(flag, "the description must not be empty")
	}

	return value, nil
}

// checkAPIURL applies the rules of the config file to the URL of an API, so that
// a value the file would refuse is refused when it is given.
func checkAPIURL(flag, value string) (string, error) {
	if err := config.CheckAPIURL(value); err != nil {
		return "", invalidFlag(flag, "%q %s", value, err)
	}

	return value, nil
}

func checkLoginURI(value string) error {
	if err := config.CheckHTTPS(value); err != nil {
		return invalidFlag("--login-uri", "%q %s", value, err)
	}

	return nil
}

// parseHints reads hints of the form name=changes:irreversible,personal_data:true.
func parseHints(values []string) (map[string]config.Hints, error) {
	hints := make(map[string]config.Hints, len(values))

	for _, value := range values {
		name, rest, found := strings.Cut(value, "=")
		name = strings.TrimSpace(name)

		if !found || name == "" {
			return nil, invalidFlag("--hint", "%q must be written <scope>=changes:<value>,personal_data:<true|false>", value)
		}

		var hint config.Hints
		var hasChanges, hasPersonalData bool

		for _, part := range strings.Split(rest, ",") {
			key, v, _ := strings.Cut(strings.TrimSpace(part), ":")

			switch key {
			case "changes":
				if v != "" && !slices.Contains(config.ValidChanges, v) {
					err := invalidFlag("--hint", "changes of %s must be one of %s, got %q", name, strings.Join(config.ValidChanges, ", "), v)
					err.Fix = fmt.Sprintf("Use --hint %s=changes:<%s>,personal_data:<true|false>", name, strings.Join(config.ValidChanges, "|"))

					return nil, err
				}
				hint.Changes, hasChanges = v, v != ""
			case "personal_data":
				b, err := strconv.ParseBool(v)
				if err != nil {
					return nil, invalidFlag("--hint", "personal_data of %s must be true or false, got %q", name, v)
				}
				hint.PersonalData, hasPersonalData = b, true
			default:
				return nil, invalidFlag("--hint", "unknown hint %q for %s: use changes and personal_data", key, name)
			}
		}

		if !hasChanges || !hasPersonalData {
			return nil, invalidFlag("--hint", "%s needs both changes and personal_data", name)
		}

		hints[name] = hint
	}

	return hints, nil
}
