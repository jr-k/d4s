package common

import (
	"strconv"
	"strings"
	"regexp"
)

// StripColorTags removes tview color tags from a string
func StripColorTags(text string) string {
	return regexp.MustCompile(`\[[^\]]*\]`).ReplaceAllString(text, "")
}

// Helper for smart comparison
func CompareValues(a, b string) bool {
	// 1. Percentage (e.g. "20.5%")
	if strings.HasSuffix(a, "%") && strings.HasSuffix(b, "%") {
		fa, errA := strconv.ParseFloat(strings.TrimSuffix(a, "%"), 64)
		fb, errB := strconv.ParseFloat(strings.TrimSuffix(b, "%"), 64)
		if errA == nil && errB == nil {
			return fa < fb
		}
	}

	// 2. Size (e.g. "10MB", "1GB") - Simple approximation
	if isSize(a) && isSize(b) {
		return parseBytes(a) < parseBytes(b)
	}

	// 3. Default String Compare
	return strings.ToLower(a) < strings.ToLower(b)
}

func isSize(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	// Must start with digit
	if s[0] < '0' || s[0] > '9' {
		return false
	}
	s = strings.ToUpper(s)
	return strings.HasSuffix(s, "B")
}

func parseBytes(s string) float64 {
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

