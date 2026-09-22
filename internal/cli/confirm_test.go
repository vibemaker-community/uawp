package cli

import (
	"strings"
	"testing"
)

func TestConfirmAcceptsOnlyExplicitAffirmative(t *testing.T) {
	for _, input := range []string{"y\n", "Y\n", "yes\n", " YES \n"} {
		approved, err := confirm(strings.NewReader(input))
		if err != nil || !approved {
			t.Fatalf("input=%q approved=%t err=%v", input, approved, err)
		}
	}
	for _, input := range []string{"n\n", "\n", "   \n", "anything\n", ""} {
		approved, err := confirm(strings.NewReader(input))
		if err != nil || approved {
			t.Fatalf("input=%q approved=%t err=%v", input, approved, err)
		}
	}
}
