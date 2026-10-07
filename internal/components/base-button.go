package components

// BaseButton is a link styled as a call to action.
type BaseButton struct {
	Href  string
	Label string
	// Variant is "secondary" for a less prominent button; anything else is
	// primary.
	Variant string
}
