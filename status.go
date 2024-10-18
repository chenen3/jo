package main

import (
	"github.com/gdamore/tcell/v2"
)

type statusBar struct {
	BaseView
	Status *bindStr
}

func newStatusBar() *statusBar {
	b := &statusBar{}
	b.height = 1
	return b
}

func (b *statusBar) FixedSize() bool { return true }

func (b *statusBar) Draw(screen tcell.Screen) {
	style := tcell.StyleDefault.Background(tcell.ColorLightGray).Foreground(tcell.ColorBlack)
	for y := b.y; y <= b.y+b.height-1; y++ {
		for x := b.x; x <= b.x+b.width-1; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}
	}

	for i, c := range b.Status.Get() {
		screen.SetContent(b.x+i, b.y, c, nil, style)
	}

	keymap := "<ctrl+s> save, <ctrl+w> close, <ctrl+q> force quit"
	for i, c := range keymap {
		if i > b.width-1 {
			break
		}
		// align right
		x := b.x + b.width - 1 - len(keymap) + i
		if x <= b.x+len(b.Status.Get()) {
			// do not cover the line number
			break
		}
		screen.SetContent(x, b.y, c, nil, style)
	}
}
