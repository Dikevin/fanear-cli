package cli

import (
	"strings"
)

// closest picks the candidate a mistyped word most likely meant: a unique
// prefix match, otherwise the nearest by edit distance within a third of the
// word's length (at least 2, and always fewer edits than the word has
// letters, so -x does not become --to). It returns "" when nothing is close.
func closest(word string, candidates []string) string {
	if word == "" {
		return ""
	}
	var prefixed []string
	for _, c := range candidates {
		if len([]rune(word)) >= 2 && strings.HasPrefix(c, word) {
			prefixed = append(prefixed, c)
		}
	}
	if len(prefixed) == 1 {
		return prefixed[0]
	}
	best, bestDist := "", 1<<30
	for _, c := range candidates {
		if d := editDistance(word, c); d < bestDist {
			best, bestDist = c, d
		}
	}
	limit := len([]rune(word)) / 3
	if limit < 2 {
		limit = 2
	}
	if bestDist <= limit && bestDist < len([]rune(word)) {
		return best
	}
	return ""
}

func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
