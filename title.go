package main

import "github.com/gdamore/tcell/v2"

type titleBar struct {
	x, y          int
	width, height int
	name          string
}

func newTitleBar(name string) *titleBar {
	return &titleBar{name: name}
}
func (t *titleBar) HandleEvent(tcell.Event) {}
func (t *titleBar) ShowCursor()             {}
func (t *titleBar) LostFocus()              {}

func (t *titleBar) SetPos(x, y, width, height int) {
	t.x = x
	t.y = y
	t.width = width
	t.height = height
}

func (t *titleBar) Pos() (x1, y1, width, height int) {
	return t.x, t.y, t.width, t.height
}

func (t *titleBar) Render() {
	style := tcell.StyleDefault.Background(tcell.ColorLightGray).Foreground(tcell.ColorBlack)
	for y := t.y; y < t.y+t.height; y++ {
		for x := t.x; x <= t.x+t.width; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}
	}

	title := t.name
	if title == "" {
		title = "untitled"
	}
	for i, c := range title {
		screen.SetContent(t.x+i, t.y, c, nil, style)
	}
}
