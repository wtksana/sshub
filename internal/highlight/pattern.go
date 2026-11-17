// Package highlight bounds regular-expression compilation shared by configuration
// validation and the output matcher.
package highlight

import (
	"errors"
	"regexp"
	"regexp/syntax"
	"unicode/utf8"
)

const (
	MaxPatternBytes      = 1024
	MaxInstructions      = 256
	MaxTotalInstructions = 1024
	maxRuneEntries       = 4096
)

// Compile rejects expensive repeats before Simplify can expand them.
func Compile(pattern string) (*regexp.Regexp, int, error) {
	if pattern == "" || len(pattern) > MaxPatternBytes || !utf8.ValidString(pattern) {
		return nil, 0, errors.New("匹配内容不能为空，且最多为 1024 个 UTF-8 字节")
	}
	tree, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, 0, err
	}
	inst, runes := estimate(tree)
	if inst+2 > MaxInstructions || runes > maxRuneEntries {
		return nil, 0, errors.New("正则表达式过于复杂，请缩短表达式或减少重复范围")
	}
	program, err := syntax.Compile(tree.Simplify())
	if err != nil {
		return nil, 0, err
	}
	if len(program.Inst) > MaxInstructions {
		return nil, 0, errors.New("正则表达式过于复杂，请简化表达式")
	}
	compiled, err := regexp.Compile(pattern)
	return compiled, len(program.Inst), err
}

func cappedAdd(a, b, limit int) int { return min(limit+1, a+b) }
func cappedMul(a, b, limit int) int {
	if a > limit || b > limit || a != 0 && b > limit/a {
		return limit + 1
	}
	return a * b
}

// Costs are conservative upper bounds; all arithmetic saturates before expansion.
func estimate(r *syntax.Regexp) (inst, runes int) {
	for _, child := range r.Sub {
		i, u := estimate(child)
		inst = cappedAdd(inst, i, MaxInstructions)
		runes = cappedAdd(runes, u, maxRuneEntries)
	}
	switch r.Op {
	case syntax.OpLiteral:
		return min(len(r.Rune), MaxInstructions+1), min(len(r.Rune), maxRuneEntries+1)
	case syntax.OpCharClass, syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		return 1, min(max(4, len(r.Rune)), maxRuneEntries+1)
	case syntax.OpCapture:
		inst = cappedAdd(inst, 2, MaxInstructions)
	case syntax.OpAlternate:
		inst = cappedAdd(inst, max(0, len(r.Sub)-1), MaxInstructions)
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest:
		inst = cappedAdd(inst, 2, MaxInstructions)
	case syntax.OpRepeat:
		n := r.Max
		if n < 0 {
			n = max(1, r.Min)
		}
		inst = cappedMul(cappedAdd(inst, 2, MaxInstructions), n, MaxInstructions)
		inst = cappedAdd(inst, 2, MaxInstructions)
		runes = cappedMul(runes, n, maxRuneEntries)
	case syntax.OpConcat:
	default:
		inst = 1
	}
	return
}
