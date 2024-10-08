package main

import (
	"log"
	"os"

	"github.com/gdamore/tcell/v2"
)

type Jo struct {
	titleBar *titleBar
	editor   *editor
	status   *statusView
	focus    View // handle event
	done     chan struct{}
	stack    *vstack // layout
}

func (j *Jo) Draw() {
	j.stack.Draw()
}

var logger *log.Logger
var screen tcell.Screen

// A multiplier to be used on the deltaX and deltaY of mouse wheel scroll events
const wheelScrollSensitivity = 0.125

func main() {
	tmp, err := os.OpenFile("/tmp/jo.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer tmp.Close()
	logger = log.New(tmp, "", log.LstdFlags|log.Lshortfile)

	s, err := tcell.NewScreen()
	if err != nil {
		logger.Print(err)
		return
	}
	if err = s.Init(); err != nil {
		logger.Print(err)
		return
	}
	screen = s
	screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset))
	screen.EnableMouse()
	screen.SetCursorStyle(tcell.CursorStyleBlinkingBlock)
	screen.EnablePaste()
	screen.Clear()
	defer screen.Fini()

	var filename string
	if len(os.Args) > 1 {
		filename = os.Args[1]
	}

	j := &Jo{
		done: make(chan struct{}),
	}
	j.titleBar = newTitleBar(j, filename)
	j.editor = newEditor(j, filename)
	j.status = &statusView{newStatusBar(j)}
	j.stack = VStack(j.titleBar, j.editor, j.status)

	j.focus = j.editor
	for {
		select {
		case <-j.done:
			return
		default:
		}

		j.status.Draw()
		j.focus.ShowCursor()
		screen.Show()

		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			width, height := screen.Size()
			j.stack.SetPos(0, 0, width, height)
			j.Draw()
			screen.Sync()
			continue
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyCtrlQ {
				_, ok := j.status.View.(*saveBar)
				if !ok && j.editor.dirty {
					j.status.Set(newSaveBar(j, true))
					j.focus = j.status
					continue
				}
				return
			}
			if ev.Key() == tcell.KeyCtrlF {
				if _, ok := j.status.View.(*findBar); !ok {
					j.status.Set(newFindBar(j))
				}
				j.focus = j.status
				break
			}
			if ev.Key() == tcell.KeyCtrlS {
				if _, ok := j.status.View.(*saveBar); !ok {
					j.status.Set(newSaveBar(j, false))
				}
				j.focus = j.status
				break
			}
			if ev.Key() == tcell.KeyCtrlP {
				if _, ok := j.status.View.(*gotoBar); !ok {
					j.status.Set(newGotoBar(j, ""))
				}
				j.focus = j.status
				break
			}
			if ev.Key() == tcell.KeyCtrlG {
				j.status.Set(newGotoBar(j, ":"))
				j.focus = j.status
				break
			}
			if ev.Key() == tcell.KeyCtrlW {
				j.titleBar.Close()
			}
		case *tcell.EventMouse:
			if ev.Buttons() == tcell.Button1 {
				x, y := ev.Position()
				for _, v := range j.stack.Views {
					if inView(v, x, y) && j.focus != v {
						j.focus.LostFocus()
						j.focus = v
					}
				}
			}
		}
		j.focus.HandleEvent(ev)
	}
}

func inView(v View, x, y int) bool {
	x1, y1, w, h := v.Pos()
	return x1 <= x && x < x1+w && y1 <= y && y < y1+h
}
