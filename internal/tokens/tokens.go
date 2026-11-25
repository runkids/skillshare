// Package tokens estimates how much of a model's context a text costs.
package tokens

import "unicode/utf8"

// Estimate approximates how many tokens s costs. ASCII text runs about
// four characters per token; anything wider (CJK, emoji, accented letters)
// is counted as one token per rune, which is what tokenizers charge for it.
// ponytail: two-bucket heuristic. Swap in a real tokenizer if a model's count
// ever needs to be exact rather than comparable.
func Estimate(s string) int {
	var ascii, wide int
	for _, r := range s {
		if r < utf8.RuneSelf {
			ascii++
		} else {
			wide++
		}
	}
	return ascii/4 + wide
}
