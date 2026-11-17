package connection

import (
	"errors"
	"fmt"
	"regexp"
	"unicode/utf8"

	"sshub/internal/highlight"
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
type HighlightRules struct {
	rules []highlightRule
	cost  int
}

type highlightBudget struct{ work, matches, extra int }

func newHighlightBudget() highlightBudget {
	return highlightBudget{work: 4 << 20, matches: 4096, extra: 48 << 10}
}

func CompileHighlightRules(input []HighlightRule) (*HighlightRules, error) {
	if len(input) > 64 {
		return nil, errors.New("日志高亮规则超过 64 条")
	}
	rules := &HighlightRules{}
	for _, v := range input {
		pattern, cost, err := highlight.Compile(v.Pattern)
		if err != nil {
			return nil, err
		}
		rules.cost += cost
		if rules.cost > highlight.MaxTotalInstructions {
			return nil, errors.New("日志高亮规则整体过于复杂")
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
	budget := newHighlightBudget()
	return r.appendBudget(dst, text, previousWord, boundary, dark, &budget)
}

func (r *HighlightRules) appendBudget(dst, text []byte, previousWord, boundary, dark bool, available *highlightBudget) ([]byte, bool) {
	if len(r.rules) == 0 {
		return append(dst, text...), true
	}
	work := r.cost * len(text)
	if !utf8.Valid(text) || work > 1<<20 || work > available.work || available.matches <= 0 {
		return append(dst, text...), false
	}
	available.work -= work
	marks := make([]uint8, len(text))
	for index, rule := range r.rules {
		matches := rule.pattern.FindAllIndex(text, available.matches+1)
		available.matches -= len(matches)
		if available.matches < 0 {
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
	extra := 0
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
			extra += len(color) + len("\x1b[39m")
		}
		i = j
	}
	if extra > available.extra {
		return append(dst, text...), false
	}
	available.extra -= extra
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
