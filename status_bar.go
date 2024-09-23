package main

import (
	"github.com/gdamore/tcell/v2"
)

type statusBar struct {
	screen tcell.Screen
	s      string
	x1, y1 int
	x2, y2 int
}

func newStatusBar(s tcell.Screen) *statusBar { return &statusBar{screen: s} }

func (b *statusBar) Draw() {
	style := tcell.StyleDefault.Background(tcell.ColorGray).Foreground(tcell.ColorWhite)
	width, height := b.screen.Size()
	b.x1, b.y1 = 0, height-1
	b.x2, b.y2 = width-1, height-1

	for y := b.y1; y <= b.y2; y++ {
		for x := b.x1; x <= b.x2; x++ {
			b.screen.SetContent(x, y, ' ', nil, style)
		}
	}

	for i, c := range b.s {
		b.screen.SetContent(b.x1+i, b.y1, c, nil, style)
	}

	text := "[ctrl+f] find | [ctrl+s] save | [ctrl+q] quit"
	for i, c := range text {
		if b.x1+i > b.x2 {
			break
		}
		// align right
		b.screen.SetContent(b.x2-len(text)+i, b.y1, c, nil, style)
	}
}

func (b *statusBar) HandleEvent(_ tcell.Event) { b.screen.HideCursor() }

func (b *statusBar) Update(s string) {
	b.s = s
}

func (b *statusBar) Position() (x1, y1, x2, y2 int) { return b.x1, b.y1, b.x2, b.y2 }
func (b *statusBar) ShowCursor()                    {}
