package layered

import (
	"sort"
	"strings"
	"unicode"
)

// stopWords are common words excluded from keyword extraction.
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true,
	"but": true, "in": true, "on": true, "at": true, "to": true,
	"for": true, "of": true, "is": true, "it": true, "that": true,
	"this": true, "with": true, "from": true, "by": true, "as": true,
	"be": true, "was": true, "are": true, "were": true, "been": true,
	"has": true, "have": true, "had": true, "not": true, "no": true,
	"do": true, "does": true, "did": true, "will": true, "would": true,
	"can": true, "could": true, "should": true, "may": true, "might": true,
	"le": true, "la": true, "les": true, "un": true, "une": true,
	"des": true, "du": true, "de": true, "et": true, "ou": true,
	"en": true, "dans": true, "sur": true, "pour": true, "par": true,
	"est": true, "sont": true, "pas": true, "que": true, "qui": true,
	"ce": true, "se": true, "ne": true, "je": true, "tu": true,
	"il": true, "elle": true, "nous": true, "vous": true, "ils": true,
}

// ExtractKeywords returns the top N keywords from text by frequency.
func ExtractKeywords(text string, max int) []string {
	tokens := tokenize(text)
	freq := make(map[string]int)
	for _, t := range tokens {
		if !stopWords[t] {
			freq[t]++
		}
	}

	type kv struct {
		word  string
		count int
	}
	pairs := make([]kv, 0, len(freq))
	for w, c := range freq {
		pairs = append(pairs, kv{w, c})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return pairs[i].word < pairs[j].word
	})

	result := make([]string, 0, max)
	for i := 0; i < len(pairs) && i < max; i++ {
		result = append(result, pairs[i].word)
	}
	return result
}

// tokenize splits text into lowercase tokens, stripping punctuation,
// keeping only tokens with 2+ characters.
func tokenize(text string) []string {
	lower := strings.ToLower(text)
	words := strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	result := make([]string, 0, len(words))
	for _, w := range words {
		if len(w) >= 2 {
			result = append(result, w)
		}
	}
	return result
}
