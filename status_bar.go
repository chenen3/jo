package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

type statusBar struct {
	jo     *Jo
	x1, y1 int
	x2, y2 int
}

func newStatusBar(j *Jo) *statusBar { return &statusBar{jo: j} }

func (b *statusBar) Draw() {
	style := tcell.StyleDefault.Background(tcell.ColorGray).Foreground(tcell.ColorWhite)
	width, height := b.jo.Size()
	b.x1, b.y1 = 0, height-1
	b.x2, b.y2 = width-1, height-1

	for y := b.y1; y <= b.y2; y++ {
		for x := b.x1; x <= b.x2; x++ {
			b.jo.SetContent(x, y, ' ', nil, style)
		}
	}

	s := fmt.Sprintf("line %d, column %d", b.jo.editor.Row(), b.jo.editor.Col())
	for i, c := range s {
		b.jo.SetContent(b.x1+i, b.y1, c, nil, style)
	}

	text := "[ctrl+f] find | [ctrl+s] save | [ctrl+q] quit"
	for i, c := range text {
		if b.x1+i > b.x2 {
			break
		}
		// align right
		b.jo.SetContent(b.x2-len(text)+i, b.y1, c, nil, style)
	}
}

func (b *statusBar) HandleEvent(_ tcell.Event) { b.jo.HideCursor() }

func (b *statusBar) Position() (x1, y1, x2, y2 int) { return b.x1, b.y1, b.x2, b.y2 }
func (b *statusBar) ShowCursor()                    {}
