package main

import (
	"log"
	"os"

	"github.com/gdamore/tcell/v2"
)

type View interface {
	Position() (x1, y1, x2, y2 int)
	Draw()
	HandleEvent(tcell.Event)
	ShowCursor()
}

type Jo struct {
	tcell.Screen
	titleBar  *titleBar
	editor    *editor
	statusBar View
	filename  string
	done      chan struct{}
	focus     View // the focused view will handle event
}

type titleBar struct {
	jo     *Jo
	x1, y1 int
	x2, y2 int
}

func (t *titleBar) Draw() {
	t.x1 = 0
	t.y1 = 0
	width, _ := t.jo.Size()
	t.x2 = width - 1
	t.y2 = 0
	style := tcell.StyleDefault.Background(tcell.ColorLightGray).Foreground(tcell.ColorBlack)
	for y := t.y1; y <= t.y2; y++ {
		for x := t.x1; x <= t.x2; x++ {
			t.jo.SetContent(x, y, ' ', nil, style)
		}
	}

	title := t.jo.filename
	if title == "" {
		title = "New Buffer"
	}
	for i, c := range title {
		t.jo.SetContent(t.x1+i, t.y1, c, nil, style)
	}
}

var logger *log.Logger

// A multiplier to be used on the deltaX and deltaY of mouse wheel scroll events
const wheelSensitivity = 0.125

func main() {
	tmp, err := os.OpenFile("/tmp/jo.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		logger.Print(err)
		return
	}
	defer tmp.Close()
	logger = log.New(tmp, "", log.LstdFlags|log.Lshortfile)

	screen, err := tcell.NewScreen()
	if err != nil {
		logger.Print(err)
		return
	}
	if err = screen.Init(); err != nil {
		logger.Print(err)
		return
	}
	screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset))
	screen.EnableMouse()
	screen.SetCursorStyle(tcell.CursorStyleBlinkingBlock)
	screen.EnablePaste()
	screen.Clear()
	defer screen.Fini()

	j := &Jo{
		Screen: screen,
		done:   make(chan struct{}),
	}

	if len(os.Args) > 1 {
		j.filename = os.Args[1]
	}

	j.titleBar = &titleBar{jo: j}
	j.titleBar.Draw()

	var src []byte
	if j.filename != "" {
		src, err = os.ReadFile(j.filename)
		if err != nil {
			logger.Print(err)
			return
		}
	}

	editor, err := newEditor(j, src)
	if err != nil {
		logger.Println(err)
		return
	}
	j.editor = editor
	j.editor.Draw()

	j.statusBar = newStatusBar(j)
	j.statusBar.Draw()

	j.focus = j.editor
	for {
		select {
		case <-j.done:
			return
		default:
		}

		j.statusBar.Draw()
		j.focus.ShowCursor()
		j.Show()

		ev := j.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			j.titleBar.Draw()
			j.statusBar.Draw()
			j.editor.Draw()
			j.Sync()
			continue
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyCtrlQ {
				_, ok := j.statusBar.(*saveBar)
				if !ok && j.editor.dirty {
					j.statusBar = newSaveBar(j, true)
					j.focus = j.statusBar
					continue
				}
				return
			}
			if ev.Key() == tcell.KeyCtrlF {
				if _, ok := j.statusBar.(*findBar); !ok {
					j.statusBar = newFindBar(j)
				}
				j.focus = j.statusBar
				break
			}
			if ev.Key() == tcell.KeyCtrlS {
				if _, ok := j.statusBar.(*saveBar); !ok {
					j.statusBar = newSaveBar(j, false)
				}
				j.focus = j.statusBar
				break
			}
			if ev.Key() == tcell.KeyCtrlP {
				if _, ok := j.statusBar.(*gotoBar); !ok {
					j.statusBar = newGotoBar(j)
				}
				j.focus = j.statusBar
				break
			}
			if ev.Key() == tcell.KeyCtrlG {
				j.statusBar = &gotoBar{jo: j, keyword: []rune{':'}} // TODO
				j.focus = j.statusBar
				break
			}
		case *tcell.EventMouse:
			if ev.Buttons() == tcell.Button1 {
				x, y := ev.Position()
				x1, y1, x2, y2 := j.editor.Position()
				if x1 <= x && x <= x2 && y1 <= y && y <= y2 {
					j.focus = j.editor
				} else {
					j.focus = j.statusBar
				}
			}
		}
		j.focus.HandleEvent(ev)
	}
}
