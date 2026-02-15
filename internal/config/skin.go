package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed skins/default.yaml
var defaultSkinYAML []byte

//go:embed skins/dracula.yaml
var draculaSkinYAML []byte

// builtinSkins maps skin names to their embedded YAML data.
var builtinSkins = map[string][]byte{
	"default": defaultSkinYAML,
	"dracula": draculaSkinYAML,
}

// Skin represents a d4s skin loaded from a YAML file.
// The structure mirrors k9s skin format, adapted for d4s.
type Skin struct {
	D4S SkinD4S `yaml:"d4s"`
}

type SkinD4S struct {
	Body  SkinBody  `yaml:"body"`
	Frame SkinFrame `yaml:"frame"`
	Views SkinViews `yaml:"views"`
}

type SkinBody struct {
	FgColor        string `yaml:"fgColor"`
	BgColor        string `yaml:"bgColor"`
	LogoColor      string `yaml:"logoColor"`
	LogoShadowColor string `yaml:"logoShadowColor"`
}

type SkinFrame struct {
	Border SkinFrameBorder `yaml:"border"`
	Crumbs SkinFrameCrumbs `yaml:"crumbs"`
	Title  SkinFrameTitle  `yaml:"title"`
	Menu   SkinFrameMenu   `yaml:"menu"`
	Status SkinFrameStatus `yaml:"status"`
}

type SkinFrameBorder struct {
	FgColor string `yaml:"fgColor"`
}

type SkinFrameCrumbs struct {
	FgColor string `yaml:"fgColor"`
	BgColor string `yaml:"bgColor"`
}

type SkinFrameTitle struct {
	FgColor        string `yaml:"fgColor"`
	HighlightColor string `yaml:"highlightColor"`
	FilterColor    string `yaml:"filterColor"`
}

type SkinFrameMenu struct {
	KeyColor string `yaml:"keyColor"`
}

type SkinFrameStatus struct {
	ErrorColor     string `yaml:"errorColor"`
	InfoColor      string `yaml:"infoColor"`
	PendingColor   string `yaml:"pendingColor"`
	WarnColor      string `yaml:"warnColor"`
	HighlightColor string `yaml:"highlightColor"`
	CompletedColor string `yaml:"completedColor"`
	DimColor       string `yaml:"dimColor"`
	PurpleColor    string `yaml:"purpleColor"`
	BlueColor      string `yaml:"blueColor"`
}

type SkinViews struct {
	Table SkinViewsTable `yaml:"table"`
}

type SkinViewsTable struct {
	CursorFgColor string          `yaml:"cursorFgColor"`
	CursorBgColor string          `yaml:"cursorBgColor"`
	MarkColor     string          `yaml:"markColor"`
	Header        SkinTableHeader `yaml:"header"`
}

type SkinTableHeader struct {
	FgColor    string `yaml:"fgColor"`
	FocusColor string `yaml:"focusColor"`
}

// builtinSkin returns an embedded skin by name, or nil if not found.
func builtinSkin(name string) *Skin {
	data, ok := builtinSkins[name]
	if !ok {
		return nil
	}
	var skin Skin
	if err := yaml.Unmarshal(data, &skin); err != nil {
		fmt.Fprintf(os.Stderr, "d4s: error: failed to parse built-in skin %q: %v\n", name, err)
		return nil
	}
	return &skin
}

// DefaultSkin returns the built-in default skin parsed from the embedded defau	lt_skin.yaml.
func DefaultSkin() *Skin {
	if skin := builtinSkin("default"); skin != nil {
		return skin
	}
	return &Skin{}
}

// loadSkinFile tries to read and parse a skin YAML from the config skins directory.
// Returns nil if the file is not found or cannot be parsed.
func loadSkinFile(skinName string) *Skin {
	dir := configDir()
	if dir == "" {
		return nil
	}

	skinPath := filepath.Join(dir, "skins", skinName+".yaml")
	data, err := os.ReadFile(skinPath)
	if err != nil {
		skinPath = filepath.Join(dir, "skins", skinName+".yml")
		data, err = os.ReadFile(skinPath)
		if err != nil {
			return nil
		}
	}

	var skin Skin
	if err := yaml.Unmarshal(data, &skin); err != nil {
		fmt.Fprintf(os.Stderr, "d4s: warning: failed to parse skin %s: %v\n", skinPath, err)
		return nil
	}

	return &skin
}

// LoadSkin loads a skin by name. Resolution order:
//  1. User file in $CONFIG_DIR/skins/<name>.yaml (or .yml)
//  2. Built-in embedded skin (default, dracula, ...)
//  3. Fall back to the built-in default skin
//
// Returns nil if the skin name is empty.
func LoadSkin(skinName string) *Skin {
	skinName = strings.TrimSpace(skinName)
	if skinName == "" {
		return nil
	}

	// 1. Try user file on disk
	if skin := loadSkinFile(skinName); skin != nil {
		return skin
	}

	// 2. Try built-in embedded skin
	if skin := builtinSkin(skinName); skin != nil {
		return skin
	}

	// 3. Fall back to built-in default
	fmt.Fprintf(os.Stderr, "d4s: warning: skin %q not found, falling back to default\n", skinName)
	return DefaultSkin()
}
