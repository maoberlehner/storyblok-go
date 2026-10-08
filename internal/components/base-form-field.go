package components

import (
	"cmp"
	"strings"

	"github.com/a-h/templ"
)

// FormControl selects how a BaseFormField renders its control.
type FormControl string

const (
	ControlInput    FormControl = "input"
	ControlTextarea FormControl = "textarea"
	ControlSelect   FormControl = "select"
	ControlCheckbox FormControl = "checkbox"
)

// BaseFormField is a labeled form control with an optional hint and error.
type BaseFormField struct {
	ID    string
	Name  string
	Label string
	Hint  string
	Error string
	// Value is the submitted value. Checkboxes are checked when it is set.
	Value        string
	Control      FormControl
	Type         string
	Autocomplete string
	Required     bool
	// Options lists select choices. An option with an empty value prompts
	// for a choice.
	Options []FormOption
	// CharacterLimit shows the remaining characters while typing, if
	// JavaScript is available. The server still validates the limit.
	CharacterLimit int
	CountMessages  CountMessages
}

// CountMessages are the remaining-characters texts, with %s for the count.
type CountMessages struct {
	RemainingOne, RemainingOther, OverOne, OverOther string
}

type FormOption struct {
	Value, Label string
}

// DescribedBy lists the IDs of the hint and error.
func (f BaseFormField) DescribedBy() string {
	var ids []string
	if f.Hint != "" {
		ids = append(ids, f.HintID())
	}
	if f.Error != "" {
		ids = append(ids, f.ErrorID())
	}
	return strings.Join(ids, " ")
}

func (f BaseFormField) HintID() string  { return f.ID + "-hint" }
func (f BaseFormField) ErrorID() string { return f.ID + "-error" }

func (f BaseFormField) InputType() string { return cmp.Or(f.Type, "text") }

// controlAttrs are the validation and description attributes every control
// carries.
func (f BaseFormField) controlAttrs() templ.OrderedAttributes {
	attrs := templ.OrderedAttributes{{Key: "required", Value: f.Required}}
	if describedBy := f.DescribedBy(); describedBy != "" {
		attrs = append(attrs, templ.KeyValue[string, any]{Key: "aria-describedby", Value: describedBy})
	}
	if f.Error != "" {
		attrs = append(attrs, templ.KeyValue[string, any]{Key: "aria-invalid", Value: "true"})
	}
	return attrs
}
