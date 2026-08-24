package display

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	managementv3 "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type formView struct {
	ID              string
	Name            string
	LanguagePrimary string
	LanguageDefault string
	NodeCount       int
	TranslationLang int
	HasStyle        bool
	CreatedAt       string
	UpdatedAt       string
	SubmittedAt     string

	raw interface{}
}

func (v *formView) AsTableHeader() []string {
	return []string{"ID", "Name", "Submitted", "Updated"}
}

func (v *formView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.SubmittedAt, v.UpdatedAt}
}

func (v *formView) KeyValues() [][]string {
	kvs := [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"LANGUAGES", formLanguageSummary(v.LanguagePrimary, v.LanguageDefault)},
		{"NODES", fmt.Sprintf("%d nodes", v.NodeCount)},
		{"TRANSLATIONS", fmt.Sprintf("%d languages", v.TranslationLang)},
		{"STYLE", boolToPresence(v.HasStyle)},
	}

	kvs = append(kvs,
		[]string{"CREATED AT", v.CreatedAt},
		[]string{"UPDATED AT", v.UpdatedAt},
	)

	if v.SubmittedAt != "" {
		kvs = append(kvs, []string{"SUBMITTED AT", v.SubmittedAt})
	}

	return kvs
}

func (v *formView) Object() interface{} {
	return v.raw
}

// formSummaryView renders a single row in the forms list.
type formSummaryView struct {
	ID          string
	Name        string
	SubmittedAt string
	UpdatedAt   string

	raw interface{}
}

func (v *formSummaryView) AsTableHeader() []string {
	return []string{"ID", "Name", "Submitted", "Updated"}
}

func (v *formSummaryView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.SubmittedAt, v.UpdatedAt}
}

func (v *formSummaryView) Object() interface{} {
	return v.raw
}

// FormsList renders the list of forms.
func (r *Renderer) FormsList(forms []*managementv3.FormSummary) error {
	resource := "forms"

	r.Heading(resource)

	if len(forms) == 0 {
		r.EmptyState(resource, "Use 'auth0 forms create' to add one")
		return nil
	}

	var res []View
	for _, f := range forms {
		res = append(res, makeFormSummaryView(f))
	}

	r.Results(res)

	return nil
}

// FormShowRaw renders a full-fidelity form response read through the v1 HTTP
// client, avoiding the v3 SDK's lossy form-node unions.
func (r *Renderer) FormShowRaw(form json.RawMessage) error {
	return r.renderRawForm("form", form)
}

// FormCreateRaw renders a full-fidelity create response.
func (r *Renderer) FormCreateRaw(form json.RawMessage) error {
	return r.renderRawForm("form created", form)
}

// FormUpdateRaw renders a full-fidelity update response.
func (r *Renderer) FormUpdateRaw(form json.RawMessage) error {
	return r.renderRawForm("form updated", form)
}

func (r *Renderer) renderRawForm(heading string, form json.RawMessage) error {
	view, err := makeFormViewFromRaw(form)
	if err != nil {
		return fmt.Errorf("failed to parse form response: %w", err)
	}
	r.Heading(heading)
	r.Result(view)
	return nil
}

func makeFormSummaryView(f *managementv3.FormSummary) *formSummaryView {
	return &formSummaryView{
		ID:          f.GetID(),
		Name:        f.GetName(),
		SubmittedAt: f.GetSubmittedAt(),
		UpdatedAt:   timeAgo(f.GetUpdatedAt()),
		raw:         mergeExtraProperties(f, f.GetExtraProperties()),
	}
}

func makeFormViewFromRaw(raw json.RawMessage) (*formView, error) {
	var form struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Languages struct {
			Primary string `json:"primary"`
			Default string `json:"default"`
		} `json:"languages"`
		Nodes        []json.RawMessage          `json:"nodes"`
		Translations map[string]json.RawMessage `json:"translations"`
		Style        json.RawMessage            `json:"style"`
		CreatedAt    time.Time                  `json:"created_at"`
		UpdatedAt    time.Time                  `json:"updated_at"`
		SubmittedAt  string                     `json:"submitted_at"`
	}
	if err := json.Unmarshal(raw, &form); err != nil {
		return nil, err
	}

	return &formView{
		ID:              form.ID,
		Name:            form.Name,
		LanguagePrimary: form.Languages.Primary,
		LanguageDefault: form.Languages.Default,
		NodeCount:       len(form.Nodes),
		TranslationLang: len(form.Translations),
		HasStyle:        rawJSONPresent(form.Style),
		CreatedAt:       rawTimeAgo(form.CreatedAt),
		UpdatedAt:       rawTimeAgo(form.UpdatedAt),
		SubmittedAt:     form.SubmittedAt,
		raw:             raw,
	}, nil
}

func rawJSONPresent(raw json.RawMessage) bool {
	value := strings.TrimSpace(string(raw))
	return value != "" && value != "null"
}

func rawTimeAgo(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return timeAgo(value)
}

// FormExport writes a form body verbatim (uncolored) to the result writer so it
// stays pipe- and import-friendly.
func (r *Renderer) FormExport(body string) {
	fmt.Fprintln(r.ResultWriter, body)
}

func formLanguageSummary(primary, def string) string {
	switch {
	case primary == "" && def == "":
		return "-"
	case def == "":
		return fmt.Sprintf("primary: %s", primary)
	case primary == "":
		return fmt.Sprintf("default: %s", def)
	default:
		return fmt.Sprintf("primary: %s, default: %s", primary, def)
	}
}

func boolToPresence(present bool) string {
	if present {
		return "set"
	}
	return "none"
}

// mergeExtraProperties rebuilds the full API wire object for JSON output. The
// generated SDK captures fields it does not model (such as flow_count and links
// on forms) into an extra-properties map that its own MarshalJSON drops, so
// re-marshaling the typed value alone would silently lose them. Marshaling the
// typed value and overlaying the extras keeps --json faithful to the API.
func mergeExtraProperties(obj interface{}, extra map[string]interface{}) interface{} {
	if len(extra) == 0 {
		return obj
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return obj
	}
	var merged map[string]interface{}
	if err := json.Unmarshal(data, &merged); err != nil {
		return obj
	}
	for key, value := range extra {
		if _, ok := merged[key]; !ok {
			merged[key] = value
		}
	}
	return merged
}
