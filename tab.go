package main

import (
	"github.com/gdamore/tcell/v2"
)

type tabBar struct {
	jo            *Jo
	x, y          int
	width, height int
	names         []string
	index         int // indicate current tab
}

func newTabBar(j *Jo, name string) *tabBar {
	t := &tabBar{jo: j, height: 1}
	if name != "" {
		t.names = append(t.names, name)
	}
	return t
}

// close current tab
func (t *tabBar) Close() {
	if len(t.names) == 0 {
		return
	}
	if len(t.names) == 1 {
		t.names = nil
		t.jo.editor.Reset()
		t.jo.Draw()
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
	t.jo.editor.Load(t.names[t.index])
	t.jo.Draw()
}

func (t *tabBar) HandleEvent(e tcell.Event) {
	m, ok := e.(*tcell.EventMouse)
	if !ok {
		return
	}
	if m.Buttons() != tcell.Button1 {
		return
	}
	x, y := m.Position()
	if y < t.y || y > t.y+t.height-1 {
		return
	}

	// on click
	var start, end int
	for i, name := range t.names {
		end = start + len(name) + len(" |")
		if start <= x && x <= end {
			if i == t.index {
				return
			}
			t.index = i
			t.jo.editor.Load(name)
			t.jo.focus = t.jo.editor
			t.jo.Draw()
			return
		}
		start = end + 1
	}
}

func (t *tabBar) ShowCursor() {}
func (t *tabBar) LostFocus()  {}
func (t *tabBar) Fixed() bool { return true }

func (t *tabBar) SetPos(x, y, width, height int) {
	t.x = x
	t.y = y
	t.width = width
}

func (t *tabBar) Pos() (x1, y1, width, height int) {
	return t.x, t.y, t.width, t.height
}

func (t *tabBar) Draw() {
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

func (t *tabBar) Add(s string) {
	for i, name := range t.names {
		if name == s {
			t.index = i
			return
		}
	}
	t.names = append(t.names, s)
	t.index = len(t.names) - 1
}
