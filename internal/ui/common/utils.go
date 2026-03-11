package common

import (
	"regexp"
	"strconv"
	"strings"
)

// StripColorTags removes tview color tags from a string
func StripColorTags(text string) string {
	return regexp.MustCompile(`\[[^\]]*\]`).ReplaceAllString(text, "")
}

// Helper for smart comparison
func CompareValues(a, b string) bool {
	// Strip colors for comparison logic
	cleanA := StripColorTags(a)
	cleanB := StripColorTags(b)

	// 0. Handle "-" (no data) — always treat as smallest value
	trimA := strings.TrimSpace(cleanA)
	trimB := strings.TrimSpace(cleanB)
	if trimA == "-" && trimB == "-" {
		return false
	}
	if trimA == "-" {
		return true
	}
	if trimB == "-" {
		return false
	}

	// 1. Percentage
	if strings.HasSuffix(cleanA, "%") && strings.HasSuffix(cleanB, "%") {
		fa, errA := strconv.ParseFloat(strings.TrimSuffix(cleanA, "%"), 64)
		fb, errB := strconv.ParseFloat(strings.TrimSuffix(cleanB, "%"), 64)
		if errA == nil && errB == nil {
			return fa < fb
		}
	}

	// 2. Size (e.g. "10MB", "1GB", "-")
	if isSize(cleanA) && isSize(cleanB) {
		return parseBytes(cleanA) < parseBytes(cleanB)
	}

	// 3. Duration/Age (e.g. "50m", "5d", "3h", "1w", "6mo", "2y")
	durA, okA := parseDuration(cleanA)
	durB, okB := parseDuration(cleanB)
	if okA && okB {
		return durA < durB
	}

	// 4. Numeric (Integers/Floats)
	// Try parsing as float to handle both simple integers and potential decimals
	fA, errA := strconv.ParseFloat(cleanA, 64)
	fB, errB := strconv.ParseFloat(cleanB, 64)
	if errA == nil && errB == nil {
		return fA < fB
	}

	// 5. Default String Compare (case insensitive on clean text)
	return strings.ToLower(cleanA) < strings.ToLower(cleanB)
}

// parseDuration converts short duration strings (from ShortenDuration) to seconds.
// Supported suffixes: s, m, h, d, w, mo, y
func parseDuration(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return 0, false
	}

	type unitDef struct {
		suffix string
		mult   float64
	}
	units := []unitDef{
		{"mo", 30 * 24 * 3600},
		{"s", 1},
		{"m", 60},
		{"h", 3600},
		{"d", 24 * 3600},
		{"w", 7 * 24 * 3600},
		{"y", 365 * 24 * 3600},
	}

	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			numStr := strings.TrimSuffix(s, u.suffix)
			val, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, false
			}
			return val * u.mult, true
		}
	}
	return 0, false
}

func isSize(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	// Special case for empty/dash
	if s == "-" {
		return true
	}
	// Must start with digit
	if s[0] < '0' || s[0] > '9' {
		return false
	}
	s = strings.ToUpper(s)
	return strings.HasSuffix(s, "B")
}

func parseBytes(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" {
		return 0
	}

	// Handle "Used / Limit" format common in docker stats
	if idx := strings.Index(s, "/"); idx != -1 {
		s = s[:idx]
	}

	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)
	unit := 1.0
	// Handle binary prefixes (KiB, MiB, etc) and decimal (KB, MB, etc)
	if strings.HasSuffix(s, "KIB") || strings.HasSuffix(s, "KB") || strings.HasSuffix(s, "K") {
		unit = 1024
	} else if strings.HasSuffix(s, "MIB") || strings.HasSuffix(s, "MB") || strings.HasSuffix(s, "M") {
		unit = 1024 * 1024
	} else if strings.HasSuffix(s, "GIB") || strings.HasSuffix(s, "GB") || strings.HasSuffix(s, "G") {
		unit = 1024 * 1024 * 1024
	} else if strings.HasSuffix(s, "TIB") || strings.HasSuffix(s, "TB") || strings.HasSuffix(s, "T") {
		unit = 1024 * 1024 * 1024 * 1024
	} else if strings.HasSuffix(s, "B") {
		unit = 1
	}
	
	// Remove all non-numeric chars except dot
	valStr := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '.' {
			return r
		}
		return -1
	}, s)
	
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0
	}
	return val * unit
}

