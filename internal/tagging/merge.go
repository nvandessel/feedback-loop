package tagging

import (
	"sort"
	"strings"

	"github.com/nvandessel/feedback-loop/internal/sanitize"
)

// MaxExtraTags is the maximum number of user-provided extra tags accepted.
// This leaves at least (MaxTags - MaxExtraTags) slots for inferred tags.
const MaxExtraTags = 5

// MergeTags merges inferred tags with user-provided extra tags.
//
// Extra tags are normalized through the dictionary (if non-nil) so that
// synonyms like "golang" are mapped to their canonical form "go". Tags not
// found in the dictionary are kept as-is (lowercased).
//
// User-provided tags have priority: they always survive the MaxTags cap.
// Inferred tags fill the remaining budget. The result is sorted, deduplicated,
// and sanitized.
//
// When extra is empty, the result is identical to the inferred input (no regression).
func MergeTags(inferred, extra []string, dict *Dictionary) []string {
	if len(extra) == 0 {
		return inferred
	}

	// Truncate extra tags to MaxExtraTags
	if len(extra) > MaxExtraTags {
		extra = extra[:MaxExtraTags]
	}

	// Normalize and deduplicate extra tags
	seen := make(map[string]bool, MaxTags)
	var userTags []string

	for _, t := range extra {
		normalized := normalizeTag(t, dict)
		if normalized == "" {
			continue
		}
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		userTags = append(userTags, normalized)
	}

	// Add inferred tags up to MaxTags budget
	budget := MaxTags - len(userTags)
	var inferredKept []string

	for _, t := range inferred {
		if budget <= 0 {
			break
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		inferredKept = append(inferredKept, t)
		budget--
	}

	// Combine and sort
	result := append(userTags, inferredKept...)
	if len(result) == 0 {
		return nil
	}
	sort.Strings(result)
	return result
}

// normalizeTag normalizes a single tag: trims whitespace, lowercases,
// looks up in dictionary for canonical form, and sanitizes.
func normalizeTag(tag string, dict *Dictionary) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" {
		return ""
	}

	// If the dictionary maps this to a canonical tag, use that
	if dict != nil {
		if canonical, ok := dict.Lookup(tag); ok {
			tag = canonical
		}
	}

	return sanitize.SanitizeBehaviorName(tag)
}
