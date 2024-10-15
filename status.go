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
	j *Jo
	baseView
}

func newStatusBar(j *Jo) *statusBar {
	b := &statusBar{j: j}
	b.height = 1
	return b
}

func (b *statusBar) FixedSize() bool { return true }

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

func (b *statusBar) OnClick(x, y int) {
	if b.j.focus == b {
		return
	}
	if b.j.focus != nil {
		b.j.focus.Defocus()
	}
	b.j.focus = b
}
