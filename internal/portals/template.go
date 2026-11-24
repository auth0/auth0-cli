package portals

// Template is the kind of portal to set up.
type Template string

const (
	// TemplateConsumer is for end users who manage their own account.
	TemplateConsumer Template = "consumer"
	// TemplateBusiness is for organization administrators who manage their organization.
	TemplateBusiness Template = "business"
)

// IsValid reports whether the template is one the package knows.
func (t Template) IsValid() bool {
	return t == TemplateConsumer || t == TemplateBusiness
}
