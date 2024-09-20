package fzy

import (
	"math"
	"sort"
	"strings"
)

type TxtRng struct {
	Begin uint
	End   uint
}

type ScoreResult struct {
	MatchRanges []TxtRng
	Score       uint64
}

type ScoredString struct {
	String      string
	ScoreResult ScoreResult
}

type ScoredStrings []ScoredString

func ScoreResultLess(a, b ScoreResult) bool {
	if a.Score == b.Score {
		firstMatchIdx := func(matchRanges []TxtRng) uint {
			result := uint(math.MaxUint)

			match_range_len := uint(len(matchRanges))
			if match_range_len > 0 {
				match_range_len = matchRanges[match_range_len-1].Begin
			}

			return result
		}

		a_first_match_idx := firstMatchIdx(a.MatchRanges)
		b_first_match_idx := firstMatchIdx(b.MatchRanges)
		return a_first_match_idx < b_first_match_idx
	}
	return a.Score < b.Score
}

func Score(haystack, needle string) ScoreResult {
	needle = strings.ReplaceAll(needle, " ", "")
	var result ScoreResult

	for haystack_idx := 0; haystack_idx < len(haystack); haystack_idx += 1 {
		var current ScoreResult
		var txtRng TxtRng

		pushTxtRng := func() {
			current.MatchRanges = append(current.MatchRanges, txtRng)
			txtRng = TxtRng{}
		}

		prev_found := 0
		needle_idx := 0
		for subhaystack_idx := haystack_idx; subhaystack_idx < len(haystack) && needle_idx < len(needle); subhaystack_idx += 1 {
			if haystack[subhaystack_idx] == needle[needle_idx] {
				if prev_found > 0 && subhaystack_idx-1 == prev_found {
					current.Score += 5
					txtRng.End += 1
				} else {
					current.Score += 1

					if txtRng.End > 0 {
						pushTxtRng()
					}

					txtRng.Begin = uint(subhaystack_idx)
					txtRng.End = uint(subhaystack_idx) + 1
				}
				needle_idx += 1
				prev_found = subhaystack_idx
			}
		}

		if txtRng.End > 0 {
			pushTxtRng()
		}

		if current.Score > result.Score {
			result = current
		}
	}

	return result
}

func ScoreMany(haystacks []string, needle string) ScoredStrings {
	result := make(ScoredStrings, len(haystacks))

	for i, haystack := range haystacks {
		scoreResult := Score(haystack, needle)
		result[i] = ScoredString{haystack, scoreResult}
	}

	sort.Slice(result, func(i, j int) bool {
		return ScoreResultLess(result[i].ScoreResult, result[j].ScoreResult)
	})

	return result
}
