package components

import "encoding/json/v2"

// BaseButton is a link styled as a call to action.
type BaseButton struct {
	Href  string
	Label string
	// Variant is "secondary" for a less prominent button; anything else is
	// primary.
	Variant string
	// Enhance loads Href with htmx instead of navigating; nil keeps a plain
	// link.
	Enhance *ButtonEnhancement
}

// ButtonEnhancement swaps the response to an enhanced request for Href into
// the page. The button shows a progress indicator during the request and
// ignores clicks until it finishes.
type ButtonEnhancement struct {
	// Block is the short ID of the block that renders the response.
	Block string
	// Target is the CSS selector of the element to swap.
	Target string
	// Swap is the htmx swap style, such as "beforeend".
	Swap string
}

// vals are the htmx request parameters that address the block.
func (e ButtonEnhancement) vals() (string, error) {
	data, err := json.Marshal(map[string]string{TargetParam: e.Block})
	return string(data), err
}
