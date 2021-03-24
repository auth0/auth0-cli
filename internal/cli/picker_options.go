package cli

type pickerOptions []pickerOption

func (p pickerOptions) labels() []string {
	result := make([]string, 0, len(p))
	for _, o := range p {
		result = append(result, o.label)
	}
	return result
}

func (p pickerOptions) defaultLabel() string {
	if len(p) > 0 {
		return p[0].label
	}
	return ""
}

func (p pickerOptions) getValue(label string) string {
	for _, o := range p {
		if o.label == label {
			return o.value
		}
	}
	return ""
}

type pickerOption struct {
	label string
	value string
}
