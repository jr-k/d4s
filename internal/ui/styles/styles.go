package styles

import "github.com/gdamore/tcell/v2"

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
	ColorDim         = tcell.NewRGBColor(98, 114, 164)  // Comment/Dim
	ColorAccent      = tcell.NewRGBColor(255, 184, 108) // Orange
	
	// Status
	ColorLogo        = tcell.NewRGBColor(255, 184, 108) // Orange
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
)

const (
	TitleContainers = "Containers"
	TitleImages     = "Images"
	TitleVolumes    = "Volumes"
	TitleNetworks   = "Networks"
	TitleServices   = "Services"
	TitleNodes      = "Nodes"
	TitleCompose    = "Compose"
	TitleAliases    = "Aliases"
)
