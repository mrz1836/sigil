package chain

import "regexp"

// txIDRegex matches a full 64-character hex transaction ID.
var txIDRegex = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// txIDExtractRegex finds the first 64-character hex string within arbitrary text
// (e.g. an error message or a response body with surrounding quotes/whitespace).
var txIDExtractRegex = regexp.MustCompile(`[0-9a-fA-F]{64}`)

// IsValidTxID reports whether s is a 64-character hex transaction ID.
func IsValidTxID(s string) bool {
	return txIDRegex.MatchString(s)
}

// ExtractTxID returns the first 64-hex substring of s, or "" if none is present.
func ExtractTxID(s string) string {
	return txIDExtractRegex.FindString(s)
}
