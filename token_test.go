package main

import (
	"reflect"
	"testing"
)

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

func Test_node_get(t *testing.T) {
	buf := [][]rune{
		[]rune("fmt.Println(1024)"),
		[]rune("println(1024)"),
	}
	n := new(node)
	buildTokenTree(n, buf)

	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{args: args{s: "fm"}, want: []string{"fmt"}},
		{name: "insensitive case", args: args{s: "pri"}, want: []string{"Println", "println"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := n.get(tt.args.s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("node.get(%q) = %v, want %v", tt.args.s, got, tt.want)
			}
		})
	}
}
