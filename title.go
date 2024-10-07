package main

import "github.com/gdamore/tcell/v2"

type titleBar struct {
	x, y          int
	width, height int
	names         []string
	index         int // index of current title
}

func newTitleBar(name string) *titleBar {
	return &titleBar{names: []string{name}, height: 1}
}
func (t *titleBar) HandleEvent(tcell.Event) {}
func (t *titleBar) ShowCursor()             {}
func (t *titleBar) LostFocus()              {}
func (t *titleBar) Fixed() bool             { return true }

func (t *titleBar) SetPos(x, y, width, height int) {
	t.x = x
	t.y = y
	t.width = width
}

func (t *titleBar) Pos() (x1, y1, width, height int) {
	return t.x, t.y, t.width, t.height
}

func (t *titleBar) Render() {
	style := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	for y := t.y; y < t.y+t.height; y++ {
		for x := t.x; x <= t.x+t.width; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}
	}

	if len(t.names) == 0 {
		for i, c := range "untitled" {
			screen.SetContent(t.x+i, t.y, c, nil, style)
		}
		return
	}

	var i int
	for j, title := range t.names {
		newstyle := style
		if j == t.index {
			newstyle = newstyle.Background(tcell.ColorLightGray).Italic(true)
		}
		for _, c := range title {
			screen.SetContent(t.x+i, t.y, c, nil, newstyle)
			i++
		}
		if j != len(t.names)-1 {
			screen.SetContent(t.x+i, t.y, ' ', nil, style)
			screen.SetContent(t.x+i+1, t.y, '|', nil, style)
			screen.SetContent(t.x+i+2, t.y, ' ', nil, style)
			i += 3
		}
	}
}

func (t *titleBar) Set(s string) {
	for i, name := range t.names {
		if name == s {
			t.index = i
			return
		}
	}
	t.names = append(t.names, s)
	t.index = len(t.names) - 1
}
