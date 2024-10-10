package main

import (
	"github.com/gdamore/tcell/v2"
)

type titleBar struct {
	e             *editor
	x, y          int
	width, height int
	names         []string
	index         int // indicate current editor
}

func newTitleBar(e *editor, name string) *titleBar {
	t := &titleBar{e: e, height: 1}
	if name != "" {
		t.names = append(t.names, name)
	}
	return t
}

func (t *titleBar) OnClick(x, y int) {
	start := t.x
	for i, name := range t.names {
		end := start + len(name) + len(" |")
		if start <= x && x <= end {
			if i == t.index {
				// clicked on current title
				t.e.Focus()
				return
			}
			t.index = i
			t.e.Load(name)
			t.e.Draw()
			t.e.Focus()
			return
		}
		start = end + 1
	}
}

// close current editor
func (t *titleBar) Close() {
	if len(t.names) == 0 {
		return
	}
	if len(t.names) == 1 {
		t.names = nil
		return
	}

	if t.index == len(t.names)-1 {
		t.names = t.names[:len(t.names)-1]
	} else {
		t.names = append(t.names[:t.index], t.names[t.index+1:]...)
	}
	if t.index > len(t.names)-1 {
		t.index = len(t.names) - 1
	}
}

func (t *titleBar) HandleEvent(e tcell.Event) {}
func (t *titleBar) ShowCursor()               {}
func (t *titleBar) Focus()                    {}
func (t *titleBar) Defocus()                  {}
func (t *titleBar) Fixed() bool               { return true }

func (t *titleBar) SetPos(x, y, width, height int) {
	t.x = x
	t.y = y
	t.width = width
}

func (t *titleBar) Pos() (x1, y1, width, height int) {
	return t.x, t.y, t.width, t.height
}

func (t *titleBar) Draw() {
	style := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	for y := t.y; y < t.y+t.height; y++ {
		for x := t.x; x < t.x+t.width; x++ {
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
	for j, name := range t.names {
		newstyle := style
		if j == t.index {
			newstyle = newstyle.Background(tcell.ColorLightGray).Italic(true)
		}
		for _, c := range name {
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

func (t *titleBar) Add(s string) {
	for i, name := range t.names {
		if name == s {
			t.index = i
			return
		}
	}
	t.names = append(t.names, s)
	t.index = len(t.names) - 1
}
