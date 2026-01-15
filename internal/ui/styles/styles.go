package styles

import "github.com/gdamore/tcell/v2"

// Indigo / Dracula-like / K9s Color Palette (Restored)
var (
	// Main Background (Indigo/Dark Blue)
	ColorBg          = tcell.NewRGBColor(30, 30, 46) // Dark Indigo
	ColorFg          = tcell.ColorWhite
	
	// Header
	ColorHeaderBg    = tcell.NewRGBColor(30, 30, 46) // Dark Indigo
	ColorHeaderFg    = tcell.NewRGBColor(139, 233, 253) // Cyan
	ColorTitle       = tcell.NewRGBColor(189, 147, 249) // Purple
	
	// Footer
	ColorFooterBg    = tcell.NewRGBColor(68, 71, 90)    // Selection Gray
	ColorFooterFg    = tcell.NewRGBColor(248, 248, 242) // White

	// Flash
	ColorFlashFg 	 = tcell.NewRGBColor(95, 135, 255) // Royal Blueish
	ColorFlashBg 	 = tcell.NewRGBColor(30, 30, 46) // Dark Indigo
	
	// Table
	ColorSelectBg    = tcell.NewRGBColor(68, 71, 90)    // Selection Gray
	ColorSelectFg    = tcell.ColorWhite
	ColorKey         = tcell.NewRGBColor(136, 192, 208) // Nord Cyan (Legacy) -> Use HeaderFg
	ColorValue       = tcell.ColorWhite
	
	// Added for compatibility with view.go
	ColorTableBorder = tcell.NewRGBColor(139, 233, 253) // Cyan
	ColorMultiSelectBg = tcell.NewRGBColor(153, 251, 152) // Pink Light/Dark BG for selection
	ColorMultiSelectFg = tcell.NewRGBColor(153, 251, 152) // Green
	
	// Text Colors
	ColorDim         = tcell.NewRGBColor(98, 114, 164)  // Comment/Dim
	ColorAccent      = tcell.NewRGBColor(255, 0, 0) // Pink
	ColorAccentAlt      = tcell.NewRGBColor(255, 184, 108) // Orange
	ColorAccentSelect = tcell.NewRGBColor(153, 251, 152) // Green
	
	// Status
	ColorLogo        = tcell.NewRGBColor(255, 184, 108) // Orange
	ColorError       = tcell.NewRGBColor(255, 85, 85)   // Red
	ColorInfo        = tcell.NewRGBColor(80, 250, 123)  // Green
	
	// Rows Status
	ColorStatusGreen = tcell.NewRGBColor(80, 250, 123)  // Green
	ColorStatusRed   = tcell.NewRGBColor(255, 85, 85)   // Red
	ColorStatusGray  = tcell.NewRGBColor(98, 114, 164)  // Gray/Purple
	ColorStatusYellow = tcell.NewRGBColor(241, 250, 140) // Yellow
	ColorStatusOrange = tcell.NewRGBColor(255, 184, 108) // Orange
	ColorStatusBlue = tcell.NewRGBColor(1, 123, 255)  // Blue (lighter)
	ColorStatusPurple = tcell.NewRGBColor(103, 35, 186) // Purple

	ColorStatusRedDarkBg = tcell.NewRGBColor(46, 30, 30) // Red
	ColorStatusGreenDarkBg = tcell.NewRGBColor(32, 46, 30)  // Green
	ColorStatusGrayDarkBg   = tcell.NewRGBColor(60, 64, 90)    // Darker Gray/Purple
	ColorStatusYellowDarkBg = tcell.NewRGBColor(46, 46, 30)  // Darker Yellow
	ColorStatusOrangeDarkBg = tcell.NewRGBColor(46, 39, 30)   // Darker Orange/Brown
	ColorStatusBlueDarkBg   = tcell.NewRGBColor(30, 37, 46)    // Darker Blue
	ColorStatusPurpleDarkBg = tcell.NewRGBColor(38, 30, 46)    // Darker Purple

	// Others
	ColorBrown = tcell.NewRGBColor(80, 50, 30) // Brown
)

const (
	TitleContainers = "Containers"
	TitleImages     = "Images"
	TitleVolumes    = "Volumes"
	TitleNetworks   = "Networks"
	TitleServices   = "Services"
	TitleNodes      = "Nodes"
	TitleCompose    = "Compose"
)
