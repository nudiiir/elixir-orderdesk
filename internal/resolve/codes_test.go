package resolve

import (
	"testing"

	"github.com/nudiiir/elixir-orderdesk/internal/rules"
)

func TestCodesAgree(t *testing.T) {
	line := rules.Profile{Words: []string{"cable", "hoco", "x21", "iphone"}}
	if codesAgree(line, rules.Profile{Words: []string{"cable", "hoco", "iph", "x21plus"}}) {
		t.Error("x21 must not be offered x21plus")
	}
	if !codesAgree(line, rules.Profile{Words: []string{"cable", "hoco", "x21", "ip"}}) {
		t.Error("the product carrying x21 must stay on the shortlist")
	}
	if !codesAgree(rules.Profile{Words: []string{"cable", "hoco"}}, rules.Profile{Words: []string{"cable", "hoco", "x21"}}) {
		t.Error("a line without a code keeps every product")
	}
}
