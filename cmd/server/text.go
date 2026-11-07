package main

import (
	"strings"
	"unicode"
)

// cleanText strips control characters and trims surrounding spaces.
func cleanText(text string) string {
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
	return strings.TrimSpace(text)
}
