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
	// Strip colors for comparison logic
	cleanA := StripColorTags(a)
	cleanB := StripColorTags(b)

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

	// 3. Default String Compare (case insensitive on clean text)
	return strings.ToLower(cleanA) < strings.ToLower(cleanB)
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

