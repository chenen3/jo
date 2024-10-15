package main

import (
	"log"
	"os"

	"github.com/gdamore/tcell/v2"
)

type Jo struct {
	editor  *editor // focused editor
	editors *hstack
	status  *statusView
	focus   View // handle event
	done    chan struct{}
	body    *vstack
	mouseX  int
	mouseY  int
}

func (j *Jo) Draw() {
	j.body.Draw()
}

var screen tcell.Screen

// A multiplier to be used on the deltaX and deltaY of mouse wheel scroll events
const wheelScrollSensitivity = 0.125

func main() {
	logFile, err := os.OpenFile("/tmp/jo.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(logFile)

	s, err := tcell.NewScreen()
	if err != nil {
		log.Print(err)
		return
	}
	if err = s.Init(); err != nil {
		log.Print(err)
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
	j.editor = newEditor(j, filename)
	j.editors = HStack(j.editor)
	j.status = &statusView{newStatusBar(j)}
	j.body = VStack(j.editors, j.status)

	width, height := screen.Size()
	j.body.SetPos(0, 0, width, height)
	j.Draw()
	j.editor.Focus()
	screen.Show()
	for {
		select {
		case <-j.done:
			return
		default:
		}

		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			width, height := screen.Size()
			j.body.SetPos(0, 0, width, height)
			j.Draw()
			screen.Sync()
			continue
		case *tcell.EventMouse:
			x, y := ev.Position()
			switch ev.Buttons() {
			case tcell.Button1:
				j.body.OnClick(x, y)
			case tcell.WheelUp:
				// scroll the editor that under the mouse, even when not being focused
				for _, v := range j.editors.Views {
					if inView(v, j.mouseX, j.mouseY) {
						delta := int(float32(y) * wheelScrollSensitivity)
						v.(*editor).scrollUp(delta)
					}
				}
			case tcell.WheelDown:
				for _, v := range j.editors.Views {
					if inView(v, j.mouseX, j.mouseY) {
						delta := int(float32(y) * wheelScrollSensitivity)
						v.(*editor).scrollDown(delta)
					}
				}
			default:
				j.mouseX = x
				j.mouseY = y
				// do not render on mouse motion
				continue
			}
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyCtrlQ {
				_, ok := j.status.View.(*saveBar)
				if !ok && j.editor.dirty {
					j.status.Set(newSaveBar(j, true))
					j.status.Draw()
					j.status.Focus()
					break
				}
				// double <ctrl+q>
				return
			}
			if ev.Key() == tcell.KeyCtrlF {
				if _, ok := j.status.View.(*findBar); !ok {
					j.focus.Defocus()
					j.status.Set(newFindBar(j))
					j.status.Draw()
				}
				j.status.Focus()
				break
			}
			if ev.Key() == tcell.KeyCtrlS {
				if _, ok := j.status.View.(*saveBar); !ok {
					j.status.Set(newSaveBar(j, false))
					j.status.Draw()
				}
				j.status.Focus()
				break
			}
			if ev.Key() == tcell.KeyCtrlP {
				if _, ok := j.status.View.(*gotoBar); !ok {
					j.status.Set(newGotoBar(j, ""))
					j.status.Draw()
				}
				j.status.Focus()
				break
			}
			if ev.Key() == tcell.KeyCtrlG {
				j.status.Set(newGotoBar(j, ":"))
				j.status.Draw()
				j.status.Focus()
				break
			}
			if ev.Key() == tcell.KeyCtrlW {
				_, ok := j.status.View.(*saveBar)
				if !ok && j.editor.dirty {
					j.status.Set(newSaveBar(j, false))
					j.status.Draw()
					j.status.Focus()
					break
				}
				// double <ctrl+w>, discard changes anyway
				j.editor.Close()
				j.Draw()
			}
		}
		j.focus.HandleEvent(ev)
		screen.Show()
	}
}
