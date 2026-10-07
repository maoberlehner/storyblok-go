package components

// BaseCard is a linked teaser, such as an article in a listing.
type BaseCard struct {
	ID    string
	Title string
	Text  string
	Href  string
	// Autofocus moves focus to the card's link when it is rendered, e.g. as
	// the first of newly loaded items.
	Autofocus bool
}
