package note

import "strings"

// QueryToken represents a single term in a parsed search query.
type QueryToken struct {
	Text  string // the search text (without # prefix or quotes)
	Tag   bool   // true if this is a #tag filter
	Exact bool   // true if this was a quoted "exact phrase"
}

// ParseQuery splits a search string into tokens.
//
// Syntax:
//   - foo bar     → two AND terms, fuzzy matched independently
//   - "foo bar"   → one exact phrase term
//   - #tag        → tag filter (matched against note tags)
//
// All tokens are AND'd together: a note must match every token.
func ParseQuery(input string) []QueryToken {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	var tokens []QueryToken
	i := 0
	for i < len(input) {
		// Skip whitespace
		for i < len(input) && input[i] == ' ' {
			i++
		}
		if i >= len(input) {
			break
		}

		switch input[i] {
		case '"':
			// Quoted phrase: scan until closing quote or end
			i++ // skip opening quote
			start := i
			for i < len(input) && input[i] != '"' {
				i++
			}
			phrase := input[start:i]
			if i < len(input) {
				i++ // skip closing quote
			}
			if phrase != "" {
				tokens = append(tokens, QueryToken{Text: strings.ToLower(phrase), Exact: true})
			}
		case '#':
			// Tag filter
			i++ // skip #
			start := i
			for i < len(input) && input[i] != ' ' {
				i++
			}
			tag := input[start:i]
			if tag != "" {
				tokens = append(tokens, QueryToken{Text: strings.ToLower(tag), Tag: true})
			}
		default:
			// Regular word
			start := i
			for i < len(input) && input[i] != ' ' {
				i++
			}
			word := input[start:i]
			if word != "" {
				tokens = append(tokens, QueryToken{Text: strings.ToLower(word)})
			}
		}
	}

	return tokens
}

// TextTokens returns only the non-tag tokens (for content/title search).
func TextTokens(tokens []QueryToken) []QueryToken {
	var result []QueryToken
	for _, t := range tokens {
		if !t.Tag {
			result = append(result, t)
		}
	}
	return result
}

// TagTokens returns only the tag filter tokens.
func TagTokens(tokens []QueryToken) []QueryToken {
	var result []QueryToken
	for _, t := range tokens {
		if t.Tag {
			result = append(result, t)
		}
	}
	return result
}
