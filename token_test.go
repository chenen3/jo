package main

import "testing"

func TestParseSyntax(t *testing.T) {
	// line := `s := "hi hello // is comment" `
	line := `i := "h\"i"`
	// line := `lastDelim == '"' || lastDelim == '"' `
	t.Logf("input: %s", line)
	tokens := parseToken([]rune(line))
	for _, token := range tokens {
		t.Logf("%s %s", line[token.off:token.off+token.len], token.class)
	}
}
