package ui

import "github.com/gdamore/tcell/v2"

// Indigo / Dracula-like / K9s Color Palette
var (
	// Main Background (Indigo/Dark Blue)
	ColorBg          = tcell.NewRGBColor(30, 30, 46) // Dark Indigo
	ColorFg          = tcell.ColorWhite
	
	// Header
	ColorHeaderBg    = tcell.NewRGBColor(30, 30, 46)
	ColorHeaderFg    = tcell.NewRGBColor(139, 233, 253) // Cyan
	ColorTitle       = tcell.NewRGBColor(189, 147, 249) // Purple
	
	// Table
	ColorSelectBg    = tcell.NewRGBColor(68, 71, 90)    // Selection Gray/Purple
	ColorSelectFg    = tcell.ColorWhite
	ColorKey         = tcell.NewRGBColor(80, 250, 123)  // Green
	ColorValue       = tcell.ColorWhite
	
	// Text Colors
	ColorDim         = tcell.NewRGBColor(98, 114, 164)  // Comment/Dim
	ColorAccent      = tcell.NewRGBColor(255, 121, 198) // Pink
	
	// Status
	ColorLogo        = tcell.NewRGBColor(255, 184, 108) // Orange
	ColorError       = tcell.NewRGBColor(255, 85, 85)   // Red
	ColorInfo        = tcell.NewRGBColor(80, 250, 123)  // Green
	
	// Rows Status
	ColorStatusGreen = tcell.NewRGBColor(80, 250, 123)  // Green
	ColorStatusRed   = tcell.NewRGBColor(255, 85, 85)   // Red
	ColorStatusGray  = tcell.NewRGBColor(98, 114, 164)  // Gray/Purple
	ColorStatusYellow = tcell.NewRGBColor(241, 250, 140) // Yellow
)

const (
	TitleContainers = "Containers"
	TitleImages     = "Images"
	TitleVolumes    = "Volumes"
	TitleNetworks   = "Networks"
	TitleServices   = "Services"
	TitleNodes      = "Nodes"
)
