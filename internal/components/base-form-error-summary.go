package components

// BaseFormErrorSummary lists all errors of a form at its top, linking to the
// invalid fields. It takes focus when rendered.
type BaseFormErrorSummary struct {
	ID     string
	Fields []BaseFormField
}
