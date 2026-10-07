package components

// BaseForm is a POST form that validates on the server. With htmx, the
// response replaces the form: the form with errors, or a success message.
type BaseForm struct {
	ID     string
	Action string
	Hidden []FormValue
	Fields []BaseFormField
	Submit string
	Guard  FormGuardFields
	// Error is a problem with the whole submission rather than one field.
	Error string
}

// ErrorSummary returns the summary of errors, or nil if there are none.
func (f BaseForm) ErrorSummary() *BaseFormErrorSummary {
	summary := BaseFormErrorSummary{ID: f.ID + "-errors", Message: f.Error}
	for _, field := range f.Fields {
		if field.Error != "" {
			summary.Fields = append(summary.Fields, field)
		}
	}
	if summary.Message == "" && len(summary.Fields) == 0 {
		return nil
	}
	return &summary
}

type FormValue struct {
	Name, Value string
}
