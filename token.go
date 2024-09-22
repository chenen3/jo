package main

import (
	gotoken "go/token"
	"slices"
	"strconv"

	"github.com/gdamore/tcell/v2"
)

// Go has four classes of tokens: identifiers(variable and types), keywords, operators and punctuation, literals
// see more in https://go.dev/ref/spec#Tokens
const (
	tcType        = "type"
	tcKeyword     = "keyword"
	tcOperator    = "operator"
	tcIntLiteral  = "int"
	tcRuneLiteral = "rune"
	tcStrLiteral  = "str"
	tcCall        = "call"
	tcComment     = "comment"
)

var (
	delimiter      = []rune{' ', '\'', '[', ']', '{', '}', '"', '\t', '\n', '.', ',', '`', '(', ')', '-', '+', '*', '&', '|', '=', '!', ':', '<', '>'}
	tokenTypes     = []string{"nil", "int", "string", "rune", "map"}
	tokenOperators = []string{"=", "+", "-", "*", "/", ">", "<", "|", "&", "!"}
	defaultColor   = tcell.ColorBlack
	colors         = map[string]tcell.Color{
		tcKeyword:     tcell.ColorPurple,
		tcType:        tcell.ColorBlue,
		tcOperator:    tcell.ColorBrown,
		tcIntLiteral:  tcell.ColorBrown,
		tcRuneLiteral: tcell.ColorPurple,
		tcStrLiteral:  tcell.ColorGreen,
		tcCall:        tcell.ColorBlue,
		tcComment:     tcell.ColorGray,
	}
)

func tokenColor(class string) tcell.Color {
	color, ok := colors[class]
	if !ok {
		color = defaultColor
	}
	return color
}

type tokenInfo struct {
	class string
	off   int // offset of the token in the line
	len   int
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
			class = tcCall
		} else if gotoken.IsKeyword(tokenS) {
			class = tcKeyword
		} else if slices.Contains(tokenTypes, tokenS) {
			class = tcType
		} else if _, err := strconv.Atoi(tokenS); err == nil {
			class = tcIntLiteral
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
					s[len(s)-1].class = tcRuneLiteral
				} else {
					s[len(s)-1].class = tcStrLiteral
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

		if !slices.Contains(delimiter, line[i]) {
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
		s = append(s, newTokenInfo(line[i:i+1], i, 0)) // delimter
		off = i + 1
		lastDelim = line[i]
	}
	return s
}
