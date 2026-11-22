package config

// Semantic colors: use these instead of raw palette names, so the UI has
// one accent for focus and selection.
const (
	// Accent marks what has the focus or is selected (borders, active tab,
	// menus, the focused button).
	Accent = Mauve
	// OnAccent is text drawn on an Accent background.
	OnAccent = Crust
	// Brand is the logo and run/history titles.
	Brand = Peach
	// Border is an idle border.
	Border = Surface2
	// Muted is secondary text.
	Muted = Overlay1
	// Go is the "do it" color (the Send button label).
	Go = Green
)

// MethodColor is the color of an HTTP method, as in the request list.
func MethodColor(method string) string {
	switch method {
	case "GET":
		return Saphire
	case "POST":
		return Green
	case "PUT":
		return Yellow
	case "PATCH":
		return Peach
	case "DELETE":
		return Red
	case "HEAD", "OPTIONS":
		return Teal
	}
	return Overlay2
}
