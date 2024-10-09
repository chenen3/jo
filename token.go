package main

import (
	"fmt"
	gotoken "go/token"
	"slices"
	"strconv"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// Go has four classes of tokens: identifiers(variable and types), keywords, operators and punctuation, literals
// see more in https://go.dev/ref/spec#Tokens
const (
	tcType        = "type"
	tcKeyword     = "keyword"
	tcOperator    = "operator"
	tcInt         = "int"
	tcRune        = "rune"
	tcString      = "str"
	tcFunction    = "func"
	tcFuncBuiltin = "funcbuiltin"
	tcComment     = "comment"
)

var (
	delimiters = []rune{
		' ', '\'', '[', ']', '{', '}', '"', '\t', '\n', '.', ',', '`', '(', ')',
		'-', '+', '*', '&', '|', '=', '!', ':', '<', '>',
	}
	tokenTypes     = []string{"nil", "int", "string", "rune", "map"}
	tokenOperators = []string{"=", "+", "-", "*", "/", ">", "<", "|", "&", "!", ":"}
	tokenFunctions = []string{
		"append", "cap", "clear", "close", "copy", "delete", "len", "make",
		"max", "min", "new", "panic", "print", "println", "recover",
	}
	defaultStyle = (tcell.Style{}).Foreground(tcell.ColorReset)
	styles       = map[string]tcell.Style{
		tcKeyword:     (tcell.Style{}).Foreground(tcell.ColorDarkRed).Italic(true),
		tcType:        (tcell.Style{}).Foreground(tcell.ColorDarkRed),
		tcOperator:    (tcell.Style{}).Foreground(tcell.ColorDarkRed),
		tcInt:         (tcell.Style{}).Foreground(tcell.ColorRoyalBlue),
		tcRune:        (tcell.Style{}).Foreground(tcell.ColorRoyalBlue),
		tcString:      (tcell.Style{}).Foreground(tcell.ColorRebeccaPurple),
		tcFunction:    (tcell.Style{}).Foreground(tcell.ColorDarkGreen),
		tcFuncBuiltin: (tcell.Style{}).Foreground(tcell.ColorRebeccaPurple),
		tcComment:     (tcell.Style{}).Foreground(tcell.ColorGray),
	}
)

type tokenInfo struct {
	class string
	off   int // offset of the token in the line
	len   int
}

func (t *tokenInfo) Style() tcell.Style {
	s, ok := styles[t.class]
	if !ok {
		return defaultStyle
	}
	return s
}

func parseToken(line []rune) []tokenInfo {
	if len(line) == 0 {
		return nil
	}

	s := make([]tokenInfo, 0, len(line))
	newTokenInfo := func(token []rune, offset int, delim rune) tokenInfo {
		tokenS := string(token)
		var class string
		if delim == '(' {
			if slices.Contains(tokenFunctions, tokenS) {
				class = tcFuncBuiltin
			} else {
				class = tcFunction
			}
		} else if gotoken.IsKeyword(tokenS) {
			class = tcKeyword
		} else if slices.Contains(tokenTypes, tokenS) {
			class = tcType
		} else if _, err := strconv.Atoi(tokenS); err == nil {
			class = tcInt
		} else if slices.Contains(tokenOperators, tokenS) {
			class = tcOperator
		}
		return tokenInfo{off: offset, len: len(token), class: class}
	}
	var token []rune
	var off = 0
	var lastDelim rune
	for i := range line {
		// string
		if lastDelim == '"' || lastDelim == '`' || lastDelim == '\'' {
			// find the next unescaped quote
			if line[i] == lastDelim && line[i-1] != '\\' {
				if lastDelim == '\'' {
					s[len(s)-1].class = tcRune
				} else {
					s[len(s)-1].class = tcString
				}
				s[len(s)-1].len = i - s[len(s)-1].off + 1
				lastDelim = 0
				off = i + 1
			}
			continue
		}

		// comment
		if line[i] == '/' && i+1 < len(line) && line[i+1] == '/' {
			s = append(s, tokenInfo{class: tcComment, off: i, len: len(line) - i})
			break
		}

		if !slices.Contains(delimiters, line[i]) {
			token = append(token, line[i])
			if i == len(line)-1 {
				s = append(s, newTokenInfo(token, off, 0))
			}
			continue
		}

		delim := line[i]
		if len(token) > 0 {
			s = append(s, newTokenInfo(token, off, delim))
			token = token[:0]
		}
		s = append(s, newTokenInfo(line[i:i+1], i, 0)) // delimiter
		off = i + 1
		lastDelim = line[i]
	}
	return s
}

func lastToken(s []rune, i int) []rune {
	for _, t := range parseToken(s) {
		if t.off <= i && i < t.off+t.len {
			token := s[t.off : t.off+t.len]
			if gotoken.IsIdentifier(string(token)) {
				return token
			}
		}
	}
	return nil
}

// a tree intended for the token
type node struct {
	value    rune
	parent   *node
	children []*node
}

func (n *node) set(s string) {
	nn := n
	for s != "" {
		c := rune(s[0])
		var ok bool
		for _, child := range nn.children {
			if child.value == c {
				nn = child
				ok = true
				break
			}
		}
		if !ok {
			newNode := &node{parent: nn, value: c}
			nn.children = append(nn.children, newNode)
			nn = newNode
		}
		s = s[1:]
	}
}

func (n *node) get(s string) []string {
	if s == "" {
		return nil
	}

	var tokens []string
	// the deepest node that matches s
	var nodes = n.children
	for _, c := range s {
		debugC := fmt.Sprintf("%c", c)
		_ = debugC
		var match []*node
		for _, node := range nodes {
			if node.value == c || unicode.ToLower(node.value) == c {
				match = append(match, node.children...)
			}
		}
		if len(match) == 0 {
			return nil
		}
		nodes = match
	}

	for _, node := range nodes {
		for _, l := range node.leafs() {
			pValues := []rune{l.value}
			for p := l.parent; p != nil && p.value != 0; p = p.parent {
				pValues = append([]rune{p.value}, pValues...)
			}
			tokens = append(tokens, string(pValues))
		}
	}
	return tokens
}

func (n *node) leafs() []*node {
	if len(n.children) == 0 {
		return []*node{n}
	}

	var ls []*node
	for _, child := range n.children {
		ls = append(ls, child.leafs()...)
	}
	return ls
}
