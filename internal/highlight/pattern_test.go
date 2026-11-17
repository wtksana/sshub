package highlight

import (
	"strings"
	"testing"
)

func TestCompileBoundsExpansionBeforeCompilation(t *testing.T) {
	for _, pattern := range []string{"", "[", `(?=x)`, `(a)\1`, `(?:a?){1000}b`, `(?:` + strings.Repeat("a", 100) + `){1000}`, strings.Repeat("x", 1025)} {
		if _, _, err := Compile(pattern); err == nil {
			t.Fatalf("accepted %q", pattern)
		}
	}
	for _, pattern := range []string{`性能`, `(?i)message`, `"id":[0-9]+`, `^`, `a?`, `\p{Han}+`, `(?:a|aa){30}b`} {
		r, cost, err := Compile(pattern)
		if err != nil || r == nil || cost < 1 || cost > MaxInstructions {
			t.Fatalf("rejected %q: %d %v", pattern, cost, err)
		}
	}
}
