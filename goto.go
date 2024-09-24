package main

import (
	"strconv"

	"github.com/gdamore/tcell/v2"
)

type gotoBar struct {
	jo               *Jo
	keyword          []rune
	x1, y1           int
	x2, y2           int
	cursorX, cursorY int
}

// TODO
// goto :line
// goto filename
// goto @symbol
func newGotoBar(j *Jo) *gotoBar { return &gotoBar{jo: j} }

func (g *gotoBar) Draw() {
	style := tcell.StyleDefault.Background(tcell.ColorLightYellow).Foreground(tcell.ColorBlack)
	width, height := g.jo.Size()
	g.x1, g.y1 = 0, height-1
	g.x2, g.y2 = width-1, height-1
	for y := g.y1; y <= g.y2; y++ {
		for x := g.x1; x <= g.x2; x++ {
			g.jo.SetContent(x, y, ' ', nil, style)
		}
	}

	s := "goto " + string(g.keyword)
	for i, c := range s {
		g.jo.SetContent(g.x1+i, g.y1, c, nil, style)
	}
	g.cursorX, g.cursorY = g.x1+len(s), g.y1
}

func (g *gotoBar) Position() (x1, y1, x2, y2 int) { return g.x1, g.y1, g.x2, g.y2 }

func (g *gotoBar) ShowCursor() {
	g.jo.ShowCursor(g.cursorX, g.cursorY)
}

func (g *gotoBar) HandleEvent(ev tcell.Event) {
	k, ok := ev.(*tcell.EventKey)
	if !ok {
		return
	}
	switch k.Key() {
	case tcell.KeyRune:
		g.keyword = append(g.keyword, k.Rune())
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(g.keyword) == 0 {
			return
		}
		g.keyword = g.keyword[:len(g.keyword)-1]
	case tcell.KeyEnter:
		if len(g.keyword) > 1 && g.keyword[0] == ':' {
			line, err := strconv.Atoi(string(g.keyword[1:]))
			if err != nil {
				logger.Printf("goto: invalid line number: %s", err)
				return
			}
			if line < 1 || line > len(g.jo.editor.buf)+1 {
				logger.Printf("goto: line number out of range")
				return
			}
			g.jo.editor.row = line
			g.jo.editor.col = 1
			if line <= g.jo.editor.height()/2 {
				g.jo.editor.startLine = 1
			} else {
				g.jo.editor.startLine = line - g.jo.editor.height()/2
			}
			g.jo.editor.Draw()
			g.jo.focus = g.jo.editor
			g.jo.statusBar = newStatusBar(g.jo)
		}
	case tcell.KeyUp:
	case tcell.KeyDown:
	case tcell.KeyESC:
		g.jo.statusBar = newStatusBar(g.jo)
		g.jo.focus = g.jo.editor
	}
}
