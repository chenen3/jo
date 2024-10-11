package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

type statusView struct {
	View
}

func (s *statusView) Set(v View) {
	x, y, w, h := s.Pos()
	v.SetPos(x, y, w, h)
	s.View = v
}

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
func (b *statusBar) Focus()      {}

func (b *statusBar) SetPos(x, y, width, height int) {
	b.x = x
	b.y = y
	b.width = width
}

func (b *statusBar) Draw() {
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

	keymap := "<ctrl+s> save, <ctrl+w> close, <ctrl+f> find, <ctrl+p> goto"
	for i, c := range keymap {
		if i > b.width-1 {
			break
		}
		// align right
		x := b.x + b.width - 1 - len(keymap) + i
		if x <= b.x+len(s) {
			// do not cover the line number
			break
		}
		screen.SetContent(x, b.y, c, nil, style)
	}
}

func (b *statusBar) HandleEvent(_ tcell.Event) { screen.HideCursor() }

func (b *statusBar) Pos() (x1, y1, width, height int) { return b.x, b.y, b.width, b.height }
func (b *statusBar) ShowCursor()                      {}
func (b *statusBar) Defocus()                         {}
func (b *statusBar) OnClick(x, y int) {
	if b.j.focus == b {
		return
	}
	if b.j.focus != nil {
		b.j.focus.Defocus()
	}
	b.j.focus = b
}
