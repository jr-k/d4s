package styles

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/config"
	"github.com/lucasb-eyer/go-colorful"
)

// Indigo / Dracula-like / K9s Color Palette (Restored)
var (
	// Main Background (Indigo/Dark Blue)
	ColorBg          = tcell.Color16 // Dark Indigo
	ColorFg          = tcell.ColorWhite
	ColorTableBorder = tcell.NewRGBColor(137, 206, 250) // Blue

	ColorBlack = tcell.Color16
	ColorWhite = tcell.NewRGBColor(255, 255, 255)

	ColorHeader = tcell.NewRGBColor(255, 255, 255) // White
	ColorHeaderFocus = tcell.NewRGBColor(255, 184, 108) // Orange

	// Header
	ColorTitle       = tcell.NewRGBColor(189, 147, 249) // Purple
	ColorIdle	 	 = tcell.NewRGBColor(137, 206, 250) // Blue
	ColorTeal 		 = tcell.NewRGBColor(94, 175, 175)  // Teal/Cyan
	
	// Footer
	ColorFooterBg    = tcell.NewRGBColor(68, 71, 90)    // Selection Gray
	ColorFooterFg    = tcell.NewRGBColor(248, 248, 242) // White

	// Flash
	ColorFlashFg 	 = tcell.NewRGBColor(95, 135, 255) // Royal Blueish
	ColorFlashBg 	 = tcell.Color16 // Dark Indigo
	
	// Table
	ColorSelectBg    = tcell.NewRGBColor(68, 71, 90)    // Selection Gray
	ColorSelectFg    = tcell.ColorWhite
	ColorValue       = tcell.ColorWhite
	
	// Added for compatibility with view.go
	ColorSelect = tcell.NewRGBColor(153, 251, 152) // Green
	
	// Text Colors
	ColorDim         = tcell.ColorDimGray  // Comment/Dim
	ColorAccent      = tcell.NewRGBColor(255, 165, 3) // Orange
	ColorAccentLight = tcell.NewRGBColor(255, 184, 108) // Light Orange
	
	// Status
	ColorLogo        = tcell.NewRGBColor(255, 184, 108) // Orange
	ColorLogoShadow  = tcell.NewRGBColor(255, 165, 3)   // Darker Orange (logo shadow)
	ColorError       = tcell.NewRGBColor(255, 85, 85)   // Red
	ColorInfo        = tcell.NewRGBColor(80, 250, 123)  // Green
	
	// Rows Status
	ColorStatusGreen = tcell.NewRGBColor(80, 250, 123)  // Green
	ColorStatusRed   = tcell.NewRGBColor(255, 85, 85)   // Red
	ColorStatusGray  = tcell.NewRGBColor(119, 136, 153)  // Gray
	ColorStatusYellow = tcell.NewRGBColor(241, 250, 140) // Yellow
	ColorStatusOrange = tcell.NewRGBColor(255, 140, 3) // Orange
	ColorStatusBlue = tcell.NewRGBColor(1, 123, 255)  // Blue (lighter)
	ColorStatusPurple = tcell.NewRGBColor(103, 35, 186) // Purple

	ColorStatusRedDarkBg = tcell.NewRGBColor(46, 30, 30) // Red
	ColorStatusGreenDarkBg = tcell.NewRGBColor(32, 46, 30)  // Green
	ColorStatusGrayDarkBg   = tcell.NewRGBColor(60, 64, 90)    // Darker Gray/Bluish
	ColorStatusYellowDarkBg = tcell.NewRGBColor(46, 46, 30)  // Darker Yellow
	ColorStatusOrangeDarkBg = tcell.NewRGBColor(46, 39, 30)   // Darker Orange/Brown
	ColorStatusBlueDarkBg   = tcell.NewRGBColor(30, 37, 46)    // Darker Blue
	ColorStatusPurpleDarkBg = tcell.NewRGBColor(38, 30, 46)    // Darker Purple

	// Tag backing colors (for proper skin/invert support)
	ColorCyanTag   = tcell.NewRGBColor(0, 255, 255)      // TagCyan
	ColorPinkTag   = tcell.NewRGBColor(255, 0, 255)       // TagPink
	ColorFilterTag = tcell.NewRGBColor(189, 147, 249)     // TagFilter
	ColorMenuKey   = tcell.NewRGBColor(32, 144, 255)      // TagSCKey
)

// Tview markup-compatible color hex strings.
// These track the tcell.Color variables above so that format strings
// like fmt.Sprintf("[%s]text", styles.TagFg) produce the correct color
// even after InvertColors() is called.
var (
	TagFg     = colorToTag(ColorFg)       // replaces [white]
	TagBg     = colorToTag(ColorBg)       // replaces [black]
	TagAccent = colorToTag(ColorAccent)   // replaces [orange]
	TagAccentLight = colorToTag(ColorAccentLight) // replaces [light orange]
	TagIdle   = colorToTag(ColorIdle)     // replaces [blue] (info blue)
	TagDim    = colorToTag(ColorDim)      // replaces [gray] / [dim]
	TagError  = colorToTag(ColorError)    // replaces [red]
	TagInfo   = colorToTag(ColorInfo)     // replaces [green]
	TagTitle  = colorToTag(ColorTitle)    // purple title
	TagLogo       = colorToTag(ColorLogo)        // logo primary
	TagLogoShadow = colorToTag(ColorLogoShadow)  // logo shadow
	TagCyan   = colorToTag(ColorCyanTag)   // breadcrumb/title cyan
	TagPink   = colorToTag(ColorPinkTag)   // scope label pink
	TagFilter = colorToTag(ColorFilterTag) // filter badge purple
	TagSCKey  = colorToTag(ColorMenuKey)   // shortcut key blue
)

// colorToTag converts a tcell.Color to a tview-compatible hex tag like "#rrggbb".
func colorToTag(c tcell.Color) string {
	r, g, b := c.RGB()
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// refreshTags re-derives all Tag* strings from the current Color* variables.
func refreshTags() {
	TagFg = colorToTag(ColorFg)
	TagBg = colorToTag(ColorBg)
	TagAccent = colorToTag(ColorAccent)
	TagAccentLight = colorToTag(ColorAccentLight)
	TagIdle = colorToTag(ColorIdle)
	TagDim = colorToTag(ColorDim)
	TagError = colorToTag(ColorError)
	TagInfo = colorToTag(ColorInfo)
	TagTitle = colorToTag(ColorTitle)
	TagLogo = colorToTag(ColorLogo)
	TagLogoShadow = colorToTag(ColorLogoShadow)
	TagCyan = colorToTag(ColorCyanTag)
	TagPink = colorToTag(ColorPinkTag)
	TagFilter = colorToTag(ColorFilterTag)
	TagSCKey = colorToTag(ColorMenuKey)
}

const (
	TitleContainers = "Containers"
	TitleImages     = "Images"
	TitleVolumes    = "Volumes"
	TitleNetworks   = "Networks"
	TitleServices   = "Services"
	TitleNodes      = "Nodes"
	TitleCompose    = "Compose"
	TitleAliases    = "Aliases"
	TitleSecrets    = "Secrets"
)

// invertColor inverts a tcell.Color by flipping its lightness while preserving hue and saturation.
func invertColor(c tcell.Color) tcell.Color {
	if c == tcell.Color16 {
		// Special case: dark terminal background -> light
		return tcell.NewRGBColor(240, 240, 240)
	}
	r, g, b := c.RGB()
	col := colorful.Color{R: float64(r) / 255.0, G: float64(g) / 255.0, B: float64(b) / 255.0}
	h, s, l := col.Hsl()
	inverted := colorful.Hsl(h, s, 1.0-l)
	ir, ig, ib := inverted.RGB255()
	return tcell.NewRGBColor(int32(ir), int32(ig), int32(ib))
}

// InvertColors flips all theme colors from dark to light or vice versa.
func InvertColors() {
	ColorBg = invertColor(ColorBg)
	ColorFg = invertColor(ColorFg)
	ColorTableBorder = invertColor(ColorTableBorder)
	ColorBlack = invertColor(ColorBlack)
	ColorWhite = invertColor(ColorWhite)
	ColorHeader = invertColor(ColorHeader)
	ColorHeaderFocus = invertColor(ColorHeaderFocus)
	ColorTitle = invertColor(ColorTitle)
	ColorIdle = invertColor(ColorIdle)
	ColorTeal = invertColor(ColorTeal)
	ColorFooterBg = invertColor(ColorFooterBg)
	ColorFooterFg = invertColor(ColorFooterFg)
	ColorFlashFg = invertColor(ColorFlashFg)
	ColorFlashBg = invertColor(ColorFlashBg)
	ColorSelectBg = invertColor(ColorSelectBg)
	ColorSelectFg = invertColor(ColorSelectFg)
	ColorValue = invertColor(ColorValue)
	ColorSelect = invertColor(ColorSelect)
	ColorDim = invertColor(ColorDim)
	ColorAccent = invertColor(ColorAccent)
	ColorLogo = invertColor(ColorLogo)
	ColorLogoShadow = invertColor(ColorLogoShadow)
	ColorError = invertColor(ColorError)
	ColorInfo = invertColor(ColorInfo)
	ColorStatusGreen = invertColor(ColorStatusGreen)
	ColorStatusRed = invertColor(ColorStatusRed)
	ColorStatusGray = invertColor(ColorStatusGray)
	ColorStatusYellow = invertColor(ColorStatusYellow)
	ColorStatusOrange = invertColor(ColorStatusOrange)
	ColorStatusBlue = invertColor(ColorStatusBlue)
	ColorStatusPurple = invertColor(ColorStatusPurple)
	ColorStatusRedDarkBg = invertColor(ColorStatusRedDarkBg)
	ColorStatusGreenDarkBg = invertColor(ColorStatusGreenDarkBg)
	ColorStatusGrayDarkBg = invertColor(ColorStatusGrayDarkBg)
	ColorStatusYellowDarkBg = invertColor(ColorStatusYellowDarkBg)
	ColorStatusOrangeDarkBg = invertColor(ColorStatusOrangeDarkBg)
	ColorStatusBlueDarkBg = invertColor(ColorStatusBlueDarkBg)
	ColorStatusPurpleDarkBg = invertColor(ColorStatusPurpleDarkBg)

	ColorCyanTag = invertColor(ColorCyanTag)
	ColorPinkTag = invertColor(ColorPinkTag)
	ColorFilterTag = invertColor(ColorFilterTag)
	ColorMenuKey = invertColor(ColorMenuKey)

	// Refresh all tag strings to match the inverted colors
	refreshTags()
}

// parseHexColor parses a hex color string like "#rrggbb" into a tcell.Color.
// Returns ok=false if the string is empty, "default", or invalid.
func parseHexColor(hex string) (tcell.Color, bool) {
	if hex == "" || hex == "default" {
		return 0, false
	}
	if hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) != 6 {
		return 0, false
	}
	var r, g, b int32
	n, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	if err != nil || n != 3 {
		return 0, false
	}
	return tcell.NewRGBColor(r, g, b), true
}

// mixColors blends fg into bg at the given ratio (0.0 = all bg, 1.0 = all fg).
func mixColors(fg, bg tcell.Color, ratio float64) tcell.Color {
	fr, fgG, fb := fg.RGB()
	br, bgG, bb := bg.RGB()
	mix := func(f, b int32) int32 {
		return b + int32(float64(f-b)*ratio)
	}
	return tcell.NewRGBColor(mix(fr, br), mix(fgG, bgG), mix(fb, bb))
}

// ApplySkin applies a skin's color definitions to all global style variables.
// Only non-empty skin fields override the defaults. When a skin is applied,
// DarkBg status colors are auto-derived from their corresponding status color
// mixed with the background.
func ApplySkin(skin *config.Skin) {
	if skin == nil {
		return
	}
	s := skin.D4S

	// Body
	if c, ok := parseHexColor(s.Body.FgColor); ok {
		ColorFg = c
		ColorValue = c
	}
	if c, ok := parseHexColor(s.Body.BgColor); ok {
		ColorBg = c
		ColorBlack = c
		ColorFlashBg = c
	}
	if c, ok := parseHexColor(s.Body.LogoColor); ok {
		ColorLogo = c
	}
	if c, ok := parseHexColor(s.Body.LogoShadowColor); ok {
		ColorLogoShadow = c
	}

	// Frame > Border
	if c, ok := parseHexColor(s.Frame.Border.FgColor); ok {
		ColorTableBorder = c
	}

	// Frame > Crumbs
	if c, ok := parseHexColor(s.Frame.Crumbs.FgColor); ok {
		ColorFooterFg = c
	}
	if c, ok := parseHexColor(s.Frame.Crumbs.BgColor); ok {
		ColorFooterBg = c
	}

	// Frame > Title
	if c, ok := parseHexColor(s.Frame.Title.FgColor); ok {
		ColorTitle = c
	}
	if c, ok := parseHexColor(s.Frame.Title.HighlightColor); ok {
		ColorAccent = c
		ColorAccentLight = c
	}
	if c, ok := parseHexColor(s.Frame.Title.FilterColor); ok {
		ColorFilterTag = c
		ColorPinkTag = c
	}

	// Frame > Menu
	if c, ok := parseHexColor(s.Frame.Menu.KeyColor); ok {
		ColorMenuKey = c
	}

	// Frame > Status
	if c, ok := parseHexColor(s.Frame.Status.ErrorColor); ok {
		ColorError = c
		ColorStatusRed = c
	}
	if c, ok := parseHexColor(s.Frame.Status.InfoColor); ok {
		ColorInfo = c
		ColorStatusGreen = c
	}
	if c, ok := parseHexColor(s.Frame.Status.PendingColor); ok {
		ColorIdle = c
		ColorTeal = c
		ColorFlashFg = c
		ColorCyanTag = c
	}
	if c, ok := parseHexColor(s.Frame.Status.WarnColor); ok {
		ColorStatusYellow = c
	}
	if c, ok := parseHexColor(s.Frame.Status.HighlightColor); ok {
		ColorStatusOrange = c
	}
	if c, ok := parseHexColor(s.Frame.Status.CompletedColor); ok {
		ColorStatusGray = c
	}
	if c, ok := parseHexColor(s.Frame.Status.DimColor); ok {
		ColorDim = c
	}
	if c, ok := parseHexColor(s.Frame.Status.PurpleColor); ok {
		ColorStatusPurple = c
	}
	if c, ok := parseHexColor(s.Frame.Status.BlueColor); ok {
		ColorStatusBlue = c
	}

	// Views > Table
	if c, ok := parseHexColor(s.Views.Table.CursorFgColor); ok {
		ColorSelectFg = c
	}
	if c, ok := parseHexColor(s.Views.Table.CursorBgColor); ok {
		ColorSelectBg = c
	}
	if c, ok := parseHexColor(s.Views.Table.MarkColor); ok {
		ColorSelect = c
	}
	if c, ok := parseHexColor(s.Views.Table.Header.FgColor); ok {
		ColorHeader = c
		ColorWhite = c
	}
	if c, ok := parseHexColor(s.Views.Table.Header.FocusColor); ok {
		ColorHeaderFocus = c
	}

	// Auto-derive DarkBg status colors from status color + background
	ColorStatusRedDarkBg = mixColors(ColorStatusRed, ColorBg, 0.15)
	ColorStatusGreenDarkBg = mixColors(ColorStatusGreen, ColorBg, 0.15)
	ColorStatusGrayDarkBg = mixColors(ColorStatusGray, ColorBg, 0.25)
	ColorStatusYellowDarkBg = mixColors(ColorStatusYellow, ColorBg, 0.15)
	ColorStatusOrangeDarkBg = mixColors(ColorStatusOrange, ColorBg, 0.15)
	ColorStatusBlueDarkBg = mixColors(ColorStatusBlue, ColorBg, 0.15)
	ColorStatusPurpleDarkBg = mixColors(ColorStatusPurple, ColorBg, 0.15)

	// Refresh all tag strings to match new colors
	refreshTags()
}
