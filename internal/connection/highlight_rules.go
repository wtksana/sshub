package connection

import (
	"errors"
	"fmt"
	"regexp"
)

// HighlightRule is a matching input, in descending priority order.
type HighlightRule struct {
	Pattern     string
	Light, Dark [3]uint8
	WholeWord   bool
}

type highlightRule struct {
	pattern     *regexp.Regexp
	light, dark string
	wholeWord   bool
}

// HighlightRules owns an immutable compiled snapshot, safe to share across streams.
type HighlightRules struct{ rules []highlightRule }

func CompileHighlightRules(input []HighlightRule) (*HighlightRules, error) {
	if len(input) > 64 {
		return nil, errors.New("日志高亮规则超过 64 条")
	}
	rules := &HighlightRules{}
	for _, v := range input {
		pattern, err := regexp.Compile(v.Pattern)
		if err != nil {
			return nil, err
		}
		color := func(c [3]uint8) string { return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c[0], c[1], c[2]) }
		rules.rules = append(rules.rules, highlightRule{pattern: pattern, light: color(v.Light), dark: color(v.Dark), wholeWord: v.WholeWord})
	}
	return rules, nil
}

func (h *HighlightStream) SetRules(rules *HighlightRules) { h.rules.Store(rules) }

// append highlights one bounded plain-text fragment. false means only its
// coloring was skipped; the caller still forwards every original byte.
func (r *HighlightRules) append(dst, text []byte, previousWord, boundary, dark bool) ([]byte, bool) {
	const maxMatches = 1024
	marks := make([]uint8, len(text))
	for index, rule := range r.rules {
		matches := rule.pattern.FindAllIndex(text, maxMatches+1)
		if len(matches) > maxMatches {
			return append(dst, text...), false
		}
		for _, match := range matches {
			start, end := match[0], match[1]
			if start == end {
				continue
			}
			if rule.wholeWord && (start == 0 && previousWord || start > 0 && asciiWord(text[start-1]) || end == len(text) && !boundary || end < len(text) && asciiWord(text[end])) {
				continue
			}
			for i := start; i < end; i++ {
				if marks[i] == 0 {
					marks[i] = uint8(index + 1)
				}
			}
		}
	}
	budget := len(text)
	for i := 0; i < len(text); {
		mark := marks[i]
		j := i + 1
		for j < len(text) && marks[j] == mark {
			j++
		}
		if mark != 0 {
			color := r.rules[mark-1].light
			if dark {
				color = r.rules[mark-1].dark
			}
			budget += len(color) + len("\x1b[39m")
		}
		i = j
	}
	if budget > 4*highlightChunk {
		return append(dst, text...), false
	}
	for i := 0; i < len(text); {
		mark := marks[i]
		j := i + 1
		for j < len(text) && marks[j] == mark {
			j++
		}
		if mark != 0 {
			color := r.rules[mark-1].light
			if dark {
				color = r.rules[mark-1].dark
			}
			dst = append(dst, color...)
		}
		dst = append(dst, text[i:j]...)
		if mark != 0 {
			dst = append(dst, "\x1b[39m"...)
		}
		i = j
	}
	return dst, true
}
