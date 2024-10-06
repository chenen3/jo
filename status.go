package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

type statusBar struct {
	j      *Jo
	x, y   int
	width  int
	height int
}

func newStatusBar(j *Jo) *statusBar {
	return &statusBar{j: j, height: 1}
}

func (b *statusBar) Fixed() bool { return true }

func (b *statusBar) SetPos(x, y, width, height int) {
	b.x = x
	b.y = y
	b.width = width
}

func (b *statusBar) Render() {
	style := tcell.StyleDefault.Background(tcell.ColorLightGray).Foreground(tcell.ColorBlack)
	for y := b.y; y <= b.y+b.height-1; y++ {
		for x := b.x; x <= b.x+b.width-1; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}
	}

	s := fmt.Sprintf("line %d, column %d", b.j.editor.Line(), b.j.editor.Column())
	for i, c := range s {
		screen.SetContent(b.x+i, b.y, c, nil, style)
	}

	text := "<ctrl+p> goto, <ctrl+f> find, <ctrl+s> save, <ctrl+q> quit"
	for i, c := range text {
		if i > b.width-1 {
			break
		}
		// align right
		screen.SetContent(b.x+b.width-1-len(text)+i, b.y, c, nil, style)
	}
}

func (b *statusBar) HandleEvent(_ tcell.Event) { screen.HideCursor() }

func (b *statusBar) Pos() (x1, y1, width, height int) { return b.x, b.y, b.width, b.height }
func (b *statusBar) ShowCursor()                      {}
func (b *statusBar) LostFocus()                       {}
