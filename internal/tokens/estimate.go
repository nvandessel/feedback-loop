// Package tokens provides token estimation utilities for the floop system.
package tokens

// EstimateTokens provides a rough token count estimate for text.
// Uses the common heuristic of ~4 characters per token for English text.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}
