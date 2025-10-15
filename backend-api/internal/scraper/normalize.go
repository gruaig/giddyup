package scraper

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// NormalizeName removes accents, punctuation, country codes
// This matches the Python normalize_name function
func NormalizeName(name string) string {
	if name == "" {
		return ""
	}

	// Remove country codes: (IRE), (GB), (FR), etc.
	if idx := strings.Index(name, "("); idx != -1 {
		name = name[:idx]
	}

	// Convert to lowercase first
	name = strings.ToLower(name)

	// Remove Roman numerals at end: "Name II" -> "Name"
	re := regexp.MustCompile(`\s+(i|ii|iii|iv|v|vi)$`)
	name = re.ReplaceAllString(name, "")

	// Remove accents using unicode normalization
	// NFD = Canonical Decomposition (separates accents from base characters)
	// Remove Mark Nonspacing (removes the accent marks)
	// NFC = Canonical Composition (recomposes characters)
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	name, _, _ = transform.String(t, name)

	// Remove punctuation
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "'", "")
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, ",", " ")

	// Collapse whitespace
	re = regexp.MustCompile(`\s+`)
	name = re.ReplaceAllString(name, " ")

	return strings.TrimSpace(name)
}

// CleanString removes common CSV-breaking characters
func CleanString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")

	// Collapse whitespace
	re := regexp.MustCompile(`\s+`)
	s = re.ReplaceAllString(s, " ")

	return strings.TrimSpace(s)
}

// NormalizeCourseName normalizes course name for matching
func NormalizeCourseName(course string) string {
	course = strings.ToLower(course)
	course = strings.TrimSpace(course)

	// Remove "AW" suffix
	course = strings.TrimSuffix(course, " (aw)")
	course = strings.TrimSuffix(course, " aw")

	// Remove common variations
	course = strings.ReplaceAll(course, " ", "")
	course = strings.ReplaceAll(course, "-", "")
	course = strings.ReplaceAll(course, "'", "")

	return course
}

// CleanRaceName removes class/grade/pattern info from race name
func CleanRaceName(raceName string) string {
	name := raceName

	// Remove Class X
	re := regexp.MustCompile(`(?i)\(?Class [A-H0-9]\)?`)
	name = re.ReplaceAllString(name, "")

	// Remove Group/Grade patterns
	re = regexp.MustCompile(`(?i)\(?(Group|Grade) [0-9IVX]+\)?`)
	name = re.ReplaceAllString(name, "")

	// Remove Listed
	name = strings.ReplaceAll(name, "Listed Race", "")
	name = strings.ReplaceAll(name, "(Listed)", "")

	return CleanString(name)
}
