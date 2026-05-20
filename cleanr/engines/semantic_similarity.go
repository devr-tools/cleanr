package engines

import (
	"math"
	"strings"
	"unicode"
)

var semanticStopwords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "by": {},
	"for": {}, "from": {}, "in": {}, "into": {}, "is": {}, "it": {}, "of": {}, "on": {},
	"or": {}, "that": {}, "the": {}, "their": {}, "then": {}, "there": {}, "these": {},
	"this": {}, "to": {}, "was": {}, "were": {}, "with": {}, "within": {},
}

type semanticFingerprint struct {
	tokens   []string
	tokenSet map[string]struct{}
	freq     map[string]float64
	bigrams  map[string]struct{}
	numbers  map[string]struct{}
}

func measureSemanticDrift(samples []string) (float64, float64) {
	if len(samples) < 2 {
		return 0, 1
	}
	total := 0.0
	count := 0.0
	for i := 0; i < len(samples); i++ {
		for j := i + 1; j < len(samples); j++ {
			total += semanticDistance(samples[i], samples[j])
			count++
		}
	}
	drift := total / count
	return drift, 1 - drift
}

func semanticDistance(a, b string) float64 {
	return 1 - semanticSimilarity(a, b)
}

func semanticSimilarity(a, b string) float64 {
	fa := buildSemanticFingerprint(a)
	fb := buildSemanticFingerprint(b)
	if len(fa.tokens) == 0 && len(fb.tokens) == 0 {
		return 1
	}
	if len(fa.tokens) == 0 || len(fb.tokens) == 0 {
		return 0
	}
	cosine := cosineSimilarity(fa.freq, fb.freq)
	tokenJaccard := setJaccard(fa.tokenSet, fb.tokenSet)
	bigramJaccard := setJaccard(fa.bigrams, fb.bigrams)
	numericSimilarity := setJaccard(fa.numbers, fb.numbers)
	tokenContainment := sequenceContainment(fa.tokenSet, fb.tokenSet)
	tokenSequence := tokenSequenceOverlap(fa.tokens, fb.tokens)
	if len(fa.numbers) == 0 && len(fb.numbers) == 0 {
		numericSimilarity = 1
	}
	similarity := 0.25*cosine + 0.15*tokenJaccard + 0.1*bigramJaccard + 0.15*numericSimilarity + 0.15*tokenContainment + 0.2*tokenSequence
	if similarity < 0 {
		return 0
	}
	if similarity > 1 {
		return 1
	}
	return similarity
}

func buildSemanticFingerprint(input string) semanticFingerprint {
	rawTokens := tokenizeSemantic(input)
	filtered := make([]string, 0, len(rawTokens))
	for _, token := range rawTokens {
		if token == "" {
			continue
		}
		if _, skip := semanticStopwords[token]; skip {
			continue
		}
		filtered = append(filtered, normalizeSemanticToken(token))
	}
	filtered = compactNonEmpty(filtered)
	fp := semanticFingerprint{
		tokens:   filtered,
		tokenSet: make(map[string]struct{}, len(filtered)),
		freq:     make(map[string]float64, len(filtered)),
		bigrams:  make(map[string]struct{}, max(len(filtered)-1, 0)),
		numbers:  make(map[string]struct{}),
	}
	for i, token := range filtered {
		fp.tokenSet[token] = struct{}{}
		fp.freq[token]++
		if isNumericToken(token) {
			fp.numbers[token] = struct{}{}
		}
		if i < len(filtered)-1 {
			fp.bigrams[token+" "+filtered[i+1]] = struct{}{}
		}
	}
	return fp
}

func tokenizeSemantic(input string) []string {
	var b strings.Builder
	b.Grow(len(input))
	for _, r := range strings.ToLower(input) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	return strings.Fields(b.String())
}

func normalizeSemanticToken(token string) string {
	if isNumericToken(token) {
		return token
	}
	switch {
	case len(token) > 4 && strings.HasSuffix(token, "ies"):
		token = strings.TrimSuffix(token, "ies") + "y"
	case len(token) > 5 && strings.HasSuffix(token, "ing"):
		token = strings.TrimSuffix(token, "ing")
	case len(token) > 4 && strings.HasSuffix(token, "ed"):
		token = strings.TrimSuffix(token, "ed")
	case len(token) > 4 && strings.HasSuffix(token, "es"):
		token = strings.TrimSuffix(token, "es")
	case len(token) > 3 && strings.HasSuffix(token, "s") && !strings.HasSuffix(token, "ss"):
		token = strings.TrimSuffix(token, "s")
	}
	return token
}

func compactNonEmpty(tokens []string) []string {
	out := tokens[:0]
	for _, token := range tokens {
		if token != "" {
			out = append(out, token)
		}
	}
	return out
}

func isNumericToken(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func setJaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	intersection := 0
	union := len(a)
	for key := range b {
		if _, ok := a[key]; ok {
			intersection++
			continue
		}
		union++
	}
	if union == 0 {
		return 1
	}
	return float64(intersection) / float64(union)
}

func sequenceContainment(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for key := range a {
		if _, ok := b[key]; ok {
			intersection++
		}
	}
	minSize := len(a)
	if len(b) < minSize {
		minSize = len(b)
	}
	if minSize == 0 {
		return 0
	}
	return float64(intersection) / float64(minSize)
}

func tokenSequenceOverlap(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	lcs := longestCommonSubsequence(a, b)
	shorter := len(a)
	if len(b) < shorter {
		shorter = len(b)
	}
	if shorter == 0 {
		return 0
	}
	return float64(lcs) / float64(shorter)
}

func longestCommonSubsequence(a, b []string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
				continue
			}
			if cur[j-1] > prev[j] {
				cur[j] = cur[j-1]
			} else {
				cur[j] = prev[j]
			}
		}
		copy(prev, cur)
		for j := range cur {
			cur[j] = 0
		}
	}
	return prev[len(b)]
}

func cosineSimilarity(a, b map[string]float64) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	dot := 0.0
	normA := 0.0
	normB := 0.0
	for key, value := range a {
		dot += value * b[key]
		normA += value * value
	}
	for _, value := range b {
		normB += value * value
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
