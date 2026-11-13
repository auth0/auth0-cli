package setup

import (
	"fmt"
	"slices"
	"strings"

	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/text"
)

// maxValueRunes is the most characters of a value from the tenant that a question
// prints.
const maxValueRunes = 300

// clean makes a value from the tenant or the user safe to print on one line.
func clean(s string) string {
	return text.Cap(text.Line(s), maxValueRunes)
}

// The parts of Poppy that a question can belong to.
const (
	PartAPI = "api"
	PartWeb = "web"
)

// Question asks for a value that is required but was not given.
type Question struct {
	// Part is PartAPI or PartWeb for a question that only matters when that part
	// is on, and empty for the others.
	Part string
	// Flag names the flag that gives the value, for callers that cannot ask.
	Flag string
	// Title names what is asked, for a caller that shows it above the question.
	Title string
	// Reason is one sentence that says why the question is asked, shown under the
	// title.
	Reason string
	// Missing says why the value is needed, for a caller that cannot ask. When it
	// is empty, Reason does.
	Missing string
	// Prompt labels the place where the answer is typed.
	Prompt  string
	Default string
	apply   func(*Inputs, string) error
}

// Answer validates the answer and stores it in the inputs.
func (q Question) Answer(in *Inputs, value string) error {
	return q.apply(in, strings.TrimSpace(value))
}

// RequiredQuestions returns a question for each of the company domain, an API and
// the web app that has not been given. A part that is off has no question.
func RequiredQuestions(in Inputs) []Question {
	var questions []Question

	if in.CompanyDomain == "" {
		questions = append(questions, companyDomainQuestion())
	}

	if len(in.APIs) == 0 && !in.NoAPI {
		questions = append(questions, Question{
			Part:   PartAPI,
			Flag:   "--api",
			Title:  "API",
			Reason: "The identifier or ID of the API that agents will use.",
			Prompt: "API",
			apply: func(in *Inputs, value string) error {
				in.APIs = append(in.APIs, value)

				return nil
			},
		})
	}

	if in.App == "" && !in.NoWeb {
		questions = append(questions, Question{
			Part:   PartWeb,
			Flag:   "--app",
			Title:  "Web app",
			Reason: "The first-party web app that does the browser join, by client ID or name.",
			Prompt: "App",
			apply: func(in *Inputs, value string) error {
				in.App = value

				return nil
			},
		})
	}

	return questions
}

func companyDomainQuestion() Question {
	return Question{
		Flag:   "--company-domain",
		Title:  "Company domain",
		Reason: "The host of your website, such as shop.example.",
		Prompt: "Domain",
		apply: func(in *Inputs, value string) (err error) {
			in.CompanyDomain, err = parseDomain(value)

			return err
		},
	}
}

func companyNameQuestion() Question {
	return Question{
		Flag:   "--company-name",
		Title:  "Company name",
		Reason: "The name agents show for your company.",
		Prompt: "Name",
		apply: func(in *Inputs, value string) error {
			if value == "" {
				return invalidFlag("--company-name", "the name must not be empty")
			}
			in.Name = value

			return nil
		},
	}
}

// apiURLQuestion asks for the OpenAPI document of an API.
func apiURLQuestion(identifier string) Question {
	return Question{
		Flag:    "--api-url " + identifier + "=<openapi url>",
		Title:   "OpenAPI document of " + clean(identifier),
		Reason:  "Agents read it to learn the API.",
		Prompt:  "URL",
		Default: openAPIURLGuess(identifier),
		apply: func(in *Inputs, value string) error {
			value, err := checkAPIURL("--api-url", value)
			if err != nil {
				return err
			}

			if in.APIURLs == nil {
				in.APIURLs = map[string]string{}
			}
			in.APIURLs[identifier] = value

			return nil
		},
	}
}

// QuestionsForMissing returns the question that gives each value a
// *config.MissingFieldsError names, so that a caller that cannot ask can name the
// flag. The company domain comes first, then the company name, then the OpenAPI
// URLs, whatever the order of the fields. A field no flag sets is returned in
// unset, as the config path it has in the file.
func QuestionsForMissing(missing *config.MissingFieldsError) (questions []Question, unset []string) {
	var domain, name, urls []Question

	for _, field := range missing.Fields {
		switch {
		case field == "organization.name":
			name = append(name, companyNameQuestion())
		case field == "organization.domain":
			domain = append(domain, companyDomainQuestion())
		case strings.HasPrefix(field, "apis[") && strings.HasSuffix(field, "].url"):
			urls = append(urls, apiURLQuestion(strings.TrimSuffix(strings.TrimPrefix(field, "apis["), "].url")))
		default:
			unset = append(unset, field)
		}
	}

	questions = append(questions, domain...)
	questions = append(questions, name...)

	return append(questions, urls...), unset
}

// openAPIURLGuess returns the usual place of the OpenAPI document of an API: at
// its origin. It is empty when the identifier is not an https URL.
func openAPIURLGuess(identifier string) string {
	if origin := config.Origin(identifier); origin != "" {
		return origin + "/openapi.json"
	}

	return ""
}

// Questions returns the next batch of questions about what the tenant state made
// necessary: the company name when the tenant, the config file and the inputs all
// lack it, the OpenAPI URL of an API that has none and the Application Login URI of the
// web app. It returns nothing when all are answered. Answer the batch, then call
// it again.
func Questions(state *State, in Inputs) []Question {
	batch := append(NameQuestions(state, in), APIURLQuestions(state, in)...)
	if len(batch) > 0 {
		return batch
	}

	return LoginURIQuestions(state, in)
}

// NameQuestions returns the question for the company name when the tenant, the
// config file and the inputs all lack it. It needs no API and no web app in the
// state.
func NameQuestions(state *State, in Inputs) []Question {
	if in.Name == "" && state.Tenant.FriendlyName == "" && (state.File == nil || state.File.Organization.Name == "") {
		return []Question{companyNameQuestion()}
	}

	return nil
}

// APIURLQuestions returns a question for the OpenAPI URL of each API of the state
// that neither the inputs nor the config file give one for. It needs no web app in
// the state.
func APIURLQuestions(state *State, in Inputs) []Question {
	var questions []Question

	for _, api := range state.APIs {
		identifier := api.Identifier

		if in.APIURLs[identifier] != "" || len(state.fileEntries(identifier)) > 0 {
			continue
		}

		questions = append(questions, apiURLQuestion(identifier))
	}

	return questions
}

// LoginURIQuestions returns the question for the Application Login URI when the web
// part is on and the web app of the state has no https one and the inputs give
// none. The state must have the web app.
func LoginURIQuestions(state *State, in Inputs) []Question {
	if in.NoWeb || in.LoginURI != "" || config.CheckHTTPS(state.App.LoginURI) == nil {
		return nil
	}

	return []Question{{
		Flag:   "--login-uri",
		Title:  "Application Login URI of " + clean(state.App.Name),
		Reason: "The page on your site that starts a sign-in. The browser join sends people there. It must be an https URL.",
		Prompt: "URL",
		apply: func(in *Inputs, value string) error {
			if err := checkLoginURI(value); err != nil {
				return err
			}
			in.LoginURI = value

			return nil
		},
	}}
}

// ScopeQuestions returns a question for each scope that is offered to agents and
// has no description, once the --scope descriptions are added. A scope on
// several APIs is asked about once, as one --scope description covers them all.
// It returns nothing when no scope lacks one, or when the scopes cannot be
// resolved for another reason, which Check reports.
func ScopeQuestions(state *State, in Inputs) []Question {
	_, undescribed, err := collectScopes(state, in, true)
	if err != nil {
		return nil
	}

	var names []string
	apis := map[string][]string{}

	for _, scope := range undescribed {
		if !slices.Contains(names, scope.name) {
			names = append(names, scope.name)
		}

		apis[scope.name] = append(apis[scope.name], scope.api)
	}

	var questions []Question

	for _, name := range names {
		questions = append(questions, Question{
			Flag:    "--scope '" + name + "=<description>'",
			Title:   "Description of " + clean(name),
			Reason:  fmt.Sprintf("Agents show it to the user before asking for approval. It is a scope of %s.", clean(strings.Join(apis[name], ", "))),
			Missing: fmt.Sprintf("%s %s no description for %s", clean(strings.Join(apis[name], ", ")), hasOrHave(len(apis[name])), clean(name)),
			Prompt:  "Text",
			apply: func(in *Inputs, value string) error {
				value, err := checkDescription("--scope", value)
				if err != nil {
					return err
				}

				if in.Scopes == nil {
					in.Scopes = map[string]string{}
				}
				in.Scopes[name] = value

				return nil
			},
		})
	}

	return questions
}

func hasOrHave(n int) string {
	if n == 1 {
		return "has"
	}

	return "have"
}
