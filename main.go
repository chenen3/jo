package main

import (
	"log"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// recent focused editor
var recentE *Editor

var screen tcell.Screen

// A multiplier to be used on the deltaX and deltaY of mouse wheel scroll events
const scrollSensitivity = 0.125

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

	statusBar := newStatusBar()
	recentE = newEditor(filename, statusBar.Status)
	editors := HStack(recentE)
	body := VStack(editors, statusBar)
	width, height := screen.Size()
	body.SetPos(0, 0, width, height)

	app := NewApp()
	app.SetBody(body)

	fb := new(findBar)
	fb.SetPos(width-40, 1, 40, 1)
	fb.Handle(tcell.KeyRune, func(k *tcell.EventKey) {
		fb.keyword = append(fb.keyword, k.Rune())
		recentE.Find(string(fb.keyword))
		fb.Draw()
	})
	fb.Handle(tcell.KeyBackspace, func(*tcell.EventKey) {
		if len(fb.keyword) == 0 {
			return
		}
		fb.keyword = fb.keyword[:len(fb.keyword)-1]
		if len(fb.keyword) == 0 {
			recentE.ClearFind()
			recentE.Draw()
		} else {
			recentE.Find(string(fb.keyword))
		}
		fb.Draw()
	})
	fb.Handle(tcell.KeyBackspace2, func(*tcell.EventKey) {
		if len(fb.keyword) == 0 {
			return
		}
		fb.keyword = fb.keyword[:len(fb.keyword)-1]
		if len(fb.keyword) == 0 {
			recentE.ClearFind()
			recentE.Draw()
		} else {
			recentE.Find(string(fb.keyword))
		}
		fb.Draw()
	})
	fb.Handle(tcell.KeyEnter, func(*tcell.EventKey) {
		recentE.FindNext()
	})
	fb.Handle(tcell.KeyDown, func(*tcell.EventKey) {
		recentE.FindNext()
	})
	fb.Handle(tcell.KeyUp, func(*tcell.EventKey) {
		recentE.FindPrev()
	})
	fb.Handle(tcell.KeyESC, func(*tcell.EventKey) {
		fb.keyword = nil
		app.Focus(recentE)
		recentE.Draw() // cover the findbar
	})

	sb := new(saveBar)
	sb.SetPos((width-40)/2, (height-3)/2, 40, 3) // align center
	sb.Handle(tcell.KeyRune, func(k *tcell.EventKey) {
		sb.name = append(sb.name, k.Rune())
		sb.Draw()
	})
	sb.Handle(tcell.KeyBackspace2, func(k *tcell.EventKey) {
		if len(sb.name) == 0 {
			return
		}
		sb.name = sb.name[:len(sb.name)-1]
		sb.Draw()
	})

	sb.Handle(tcell.KeyEnter, func(k *tcell.EventKey) {
		if len(sb.name) == 0 {
			return
		}
		f, err := os.Create(string(sb.name))
		if err != nil {
			log.Print(err)
			return
		}
		defer f.Close()
		_, err = recentE.WriteTo(f)
		if err != nil {
			log.Print(err)
			return
		}

		recentE.Load(string(sb.name))
		app.Focus(recentE)
		recentE.Draw()
	})
	sb.Handle(tcell.KeyESC, func(k *tcell.EventKey) {
		sb.name = nil
		app.Focus(recentE)
		recentE.Draw() // cover the savebar
	})

	gb := newGotoBar()
	gb.Handle(tcell.KeyEsc, func(*tcell.EventKey) {
		gb.keyword = nil
		recentE.Draw()
		app.Focus(recentE)
	})
	gb.Handle(tcell.KeyRune, func(k *tcell.EventKey) {
		defer gb.Draw()
		recentE.Draw() // clear previous options
		gb.keyword = append(gb.keyword, k.Rune())
		if gb.keyword[0] == ':' {
			return
		}
		options := make([]string, 0, len(files))
		for _, f := range files {
			if strings.Contains(strings.ToLower(f), string(gb.keyword)) {
				options = append(options, f)
			}
		}
		gb.options = options
	})
	gb.Handle(tcell.KeyBackspace2, func(*tcell.EventKey) {
		if len(gb.keyword) == 0 {
			return
		}
		defer gb.Draw()
		recentE.Draw() // clear previous options
		gb.keyword = gb.keyword[:len(gb.keyword)-1]
		if len(gb.keyword) == 0 {
			return
		}
		if gb.keyword[0] == ':' {
			return
		}
		options := make([]string, 0, len(files))
		for _, f := range files {
			if strings.Contains(strings.ToLower(f), string(gb.keyword)) {
				options = append(options, f)
			}
		}
		gb.options = options
	})
	gb.Handle(tcell.KeyEnter, func(*tcell.EventKey) {
		// go to line
		if len(gb.keyword) > 0 && gb.keyword[0] == ':' {
			line, err := strconv.Atoi(string(gb.keyword[1:]))
			if err != nil {
				log.Printf("goto: invalid line number: %s", err)
				return
			}
			if line < 1 || line > len(recentE.buf)+1 {
				log.Printf("goto: line number out of range")
				return
			}
			recentE.line = line
			recentE.column = 1
			if line <= recentE.PageSize()/2 {
				recentE.startLine = 1
			} else {
				recentE.startLine = line - recentE.PageSize()/2
			}
			recentE.Draw()
			app.Focus(recentE)
			gb.index = 0
			gb.keyword = nil
			return
		}
		// go to file
		if len(gb.options) > 0 {
			recentE.Load(gb.options[gb.index])
			recentE.Draw()
			app.Focus(recentE)
			gb.keyword = nil
			gb.index = 0
		}
	})
	gb.Handle(tcell.KeyUp, func(*tcell.EventKey) {
		gb.index--
		if gb.index < 0 {
			gb.index = len(files) - 1
		}
		gb.Draw()
	})
	gb.Handle(tcell.KeyDown, func(*tcell.EventKey) {
		gb.index++
		if gb.index > len(files)-1 {
			gb.index = 0
		}
		gb.Draw()
	})
	gb.Handle(tcell.KeyCtrlBackslash, func(*tcell.EventKey) {
		e := newEditor(gb.options[gb.index], statusBar.Status)
		editors.Views = append(editors.Views, e)
		app.Focus(e)
		app.Redraw()
	})

	app.Handle(tcell.KeyCtrlQ, func(*tcell.EventKey) {
		// force quit
		app.Close()
	})
	app.Handle(tcell.KeyCtrlF, func(*tcell.EventKey) {
		app.Focus(fb)
		fb.Draw()
	})
	app.Handle(tcell.KeyCtrlS, func(*tcell.EventKey) {
		if !recentE.dirty {
			return
		}
		if recentE.filename == "" {
			app.Focus(sb)
			sb.Draw()
		} else {
			f, err := os.Create(recentE.filename)
			if err != nil {
				log.Print(err)
				return
			}
			_, err = recentE.WriteTo(f)
			f.Close()
			if err != nil {
				log.Print(err)
				return
			}
		}
	})
	app.Handle(tcell.KeyCtrlP, func(*tcell.EventKey) {
		gb.Draw()
		app.Focus(gb)
	})
	app.Handle(tcell.KeyCtrlG, func(*tcell.EventKey) {
		gb.keyword = []rune{':'}
		gb.Draw()
		app.Focus(gb)
	})
	app.Handle(tcell.KeyCtrlW, func(*tcell.EventKey) {
		if recentE.dirty {
			app.Focus(sb)
			sb.Draw()
			return
		}

		recentE.CloseBuffer()
		if len(recentE.titleBar.names) > 0 {
			return
		}

		if len(editors.Views) == 1 {
			app.Close()
			return
		}

		// delete editor
		var i int
		for i = range editors.Views {
			if editors.Views[i] == recentE {
				break
			}
		}
		editors.Views = slices.Delete(editors.Views, i, i+1)
		j := i - 1
		if j < 0 {
			j = 0
		}
		prevE := editors.Views[j].(*Editor)
		app.Focus(prevE)
		app.Redraw()
	})
	app.Focus(recentE)
	app.Run()
}
