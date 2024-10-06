package main

import (
	"log"
	"os"

	"github.com/gdamore/tcell/v2"
)

type Jo struct {
	stack     *vstack
	titleBar  *titleBar
	editor    *editor
	statusBar View
	focus     View // handle event
	done      chan struct{}
}

func (j *Jo) replaceStatus(v View) {
	x, y, w, h := j.statusBar.Pos()
	v.SetPos(x, y, w, h)
	j.statusBar = v
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
	j.titleBar = newTitleBar(filename)
	j.editor = newEditor(filename)
	j.statusBar = newStatusBar(j)
	j.stack = VStack(j.titleBar, j.editor, j.statusBar)

	j.focus = j.editor
	for {
		select {
		case <-j.done:
			return
		default:
		}

		j.statusBar.Render()
		j.focus.ShowCursor()
		screen.Show()

		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			width, height := screen.Size()
			j.stack.SetPos(0, 0, width, height)
			j.stack.Render()
			screen.Sync()
			continue
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyCtrlQ {
				_, ok := j.statusBar.(*saveBar)
				if !ok && j.editor.dirty {
					j.replaceStatus(newSaveBar(j, true))
					j.focus = j.statusBar
					continue
				}
				return
			}
			if ev.Key() == tcell.KeyCtrlF {
				if _, ok := j.statusBar.(*findBar); !ok {
					j.replaceStatus(newFindBar(j))
				}
				j.focus = j.statusBar
				break
			}
			if ev.Key() == tcell.KeyCtrlS {
				if _, ok := j.statusBar.(*saveBar); !ok {
					j.replaceStatus(newSaveBar(j, false))
				}
				j.focus = j.statusBar
				break
			}
			if ev.Key() == tcell.KeyCtrlP {
				if _, ok := j.statusBar.(*gotoBar); !ok {
					j.replaceStatus(newGotoBar(j, ""))
				}
				j.focus = j.statusBar
				break
			}
			if ev.Key() == tcell.KeyCtrlG {
				j.replaceStatus(newGotoBar(j, ":"))
				j.focus = j.statusBar
				break
			}
			if ev.Key() == tcell.KeyESC {
				j.editor.ClearFind()
				j.editor.Render()
				j.replaceStatus(newStatusBar(j))
				j.focus = j.editor
				break
			}
		case *tcell.EventMouse:
			if ev.Buttons() == tcell.Button1 {
				x, y := ev.Position()
				if inView(j.editor, x, y) && j.focus != j.editor {
					j.focus.LostFocus()
					j.focus = j.editor
				}
				if inView(j.statusBar, x, y) && j.focus != j.statusBar {
					j.focus.LostFocus()
					j.focus = j.statusBar
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
