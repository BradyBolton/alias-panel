package panel

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	log "github.com/sirupsen/logrus"

	"github.com/BradyBolton/alias-panel/parser"
)

const (
	minWindowWidth  = 10
	minWindowHeight = 10
	minPanelWidth   = 40
	maxPanelWidth   = 50
)

// emitStr prints str on screen s at (x,y) in style st.
func emitStr(s tcell.Screen, x, y int, style tcell.Style, str string) {
	for _, c := range str {
		var comb []rune
		w := runewidth.RuneWidth(c)
		if w == 0 {
			comb = []rune{c}
			c = ' '
			w = 1
		}
		s.SetContent(x, y, c, comb, style)
		x += w
	}
	s.Show()
}

// drawBox draws a (lw x lh) box on screen s with the UL corner at (lx, ly).
func drawBox(s tcell.Screen, lx, ly, lw, lh int, st tcell.Style) {
	x1 := lx
	x2 := lx + lw - 1
	y1 := ly
	y2 := ly + lh - 1

	for col := x1; col <= x2; col++ {
		s.SetContent(col, y1, tcell.RuneHLine, nil, st)
		s.SetContent(col, y2, tcell.RuneHLine, nil, st)
	}
	for row := y1 + 1; row < y2; row++ {
		s.SetContent(x1, row, tcell.RuneVLine, nil, st)
		s.SetContent(x2, row, tcell.RuneVLine, nil, st)
	}
	s.SetContent(x1, y1, tcell.RuneULCorner, nil, st)
	s.SetContent(x2, y1, tcell.RuneURCorner, nil, st)
	s.SetContent(x1, y2, tcell.RuneLLCorner, nil, st)
	s.SetContent(x2, y2, tcell.RuneLRCorner, nil, st)

	s.Show()
}

// truncate will shorten s to fit within w characters, appending "..." at the
// end of the result if possible.
func truncate(s string, w int) (string, error) {
	if w < 0 {
		return "", errors.New("w cannot be negative")
	}

	// No truncation required
	if len(s) <= w {
		return s, nil
	}

	// Truncate string, appending an (...) if possible
	rs := []rune(s)
	var b strings.Builder
	b.Grow(w)
	i := 0
	for ; i < b.Cap()-3; i++ {
		b.WriteRune(rs[i])
	}

	if b.Len() > 0 {
		b.WriteString("...")
	} else {
		for ; i < b.Cap(); i++ {
			b.WriteRune(rs[i])
		}
	}
	return b.String(), nil
}

// drawTextBox will draw a body text t to fit within a (w x h) area, located
// at (x, y) on screen s (in style st).
func drawTextBox(s tcell.Screen, x, y, w, h int, st tcell.Style, t string) error {
	if w < 0 || h < 0 {
		return errors.New("w and h cannot be negative")
	}

	tt, err := truncate(t, h*w)
	if err != nil {
		return err
	}

	rs := []rune(tt)
	i := 0

loop:
	for i < len(tt) { // iterate per line
		var b strings.Builder
		b.Grow(w)
		for j := 0; j < w; j++ {
			if i >= len(tt) {
				emitStr(s, x, y, st, b.String())
				break loop
			}
			b.WriteRune(rs[i])
			i++
		}
		emitStr(s, x, y, st, b.String())
		y++
	}
	return nil
}

// minHeight reports the minimum height required to display t text within a
// horizontal span of w spaces.
func minHeight(w int, t string) int {
	h := len(t) / w
	if (len(t) % w) > 0 {
		h++
	}
	return h
}

// drawSection draws a single (w x h) sized box for Section, with the UL corner
// at (x, y), a label, and body text.
func drawSection(s tcell.Screen, x, y, w, h int, sn parser.Section) error {
	if x < 0 || y < 0 || w < 0 || h < 0 {
		return errors.New("x, y, w, and h cannot be negative")
	}

	// Draw frame
	st := tcell.StyleDefault.
		Foreground(tcell.ColorWhite)
	drawBox(s, x, y, w, h, st)

	// Draw label
	ltext, err := truncate(sn.Label, w-4)
	if err != nil {
		log.Error(err)
		return err
	}
	label := fmt.Sprintf("[%v]", ltext)
	hpadding := (w - len(label)) / 2
	lx := x + hpadding
	st = tcell.StyleDefault.
		Foreground(tcell.ColorRed)
	emitStr(s, lx, y, st, label)
	s.Show()

	// Draw body text
	ax := x + 1
	ay := y + 1
	aw := w - 2
	var ah int
	for _, a := range sn.Aliases {
		btext := a.Name + ": " + a.Cmd
		ah = minHeight(aw, btext)

		// Short-circuit if this alias goes out of bounds
		if (ay+ah)-y >= h {
			break
		}

		// Otherwise print the alias
		err := drawTextBox(s, ax, ay, aw, ah, st, btext)
		if err != nil {
			log.Errorf("drawSection: Issue parsing (%v)", err)
		}
		ay += ah
	}

	return nil
}

// maxColumns dynamically calculates the number of columns a w-wide window
// can support, for a minimum and maximum width miw and maw (respectively).
func maxColumns(w, miw, maw int) int {
	// TODO: Account for illegal cases?

	// NOTE: We assume that margins are 2 spaces. (In general, even-numbered
	// margins are less messier to work with.)

	as := w - 2          // available (horizontal) space
	x1 := as / (miw + 2) // deliberate integer division
	x2 := as / (maw + 2)
	if x1 > x2 {
		return x1
	}
	return x2
}

// Return the number of vertical panels a window of width w can support and
// recommend a width for each panel given margin m in between each panel.
func maxPanels(w, m int) (int, int) {
	np := maxColumns(w, minPanelWidth, maxPanelWidth)

	// Terminal window is only large enough for one column. In this case,
	// we allow the single column to be resized smaller than usual
	if np == 0 {
		np = 1
	}

	pw := (w - (1+np)*m) / np
	return np, pw
}

// Draw panels for in the terminal, one for each section in Section map sm.
func drawPanels(s tcell.Screen, sm map[string]parser.Section) {
	w, h := s.Size()

	// Render nothing if space is too small
	if w < minWindowWidth || h < minWindowHeight {
		return
	}

	m := 2 // TODO: Make this configurable (optional argument)?
	np, pw := maxPanels(w, m)
	var ph int
	px := m
	py := m

	ss := make([]parser.Section, 0)
	for _, v := range sm {
		ss = append(ss, v)
	}

	// TODO: Stack panels column-wise (and handle cut-offs)
	// TODO: Implement a work-around for undefined order for maps
	for p := 0; p < np; p++ {
		ph = 0
		for _, a := range ss[p].Aliases {
			btext := a.Name + ": " + a.Cmd
			ph += minHeight(pw-2, btext)
		}
		log.Infof("ph (%v): %v", ss[p].Label, ph)
		err := drawSection(s, px, py, pw, ph+2, ss[p])
		if err != nil {
			log.Error(err)
			return
		}
		px += pw + m
	}
}

// Draw panels on screen for sections in Section map sm.
func DrawScreen(sm map[string]parser.Section) {
	tcell.SetEncodingFallback(tcell.EncodingFallbackASCII)

	s, err := tcell.NewScreen()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	if err = s.Init(); err != nil {
		log.Error(err)
		os.Exit(1)
	}

	s.SetStyle(tcell.StyleDefault.
		Foreground(tcell.ColorWhite))
	s.Clear()

	quit := make(chan struct{})
	go func() {
		for {
			ev := s.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventKey:
				switch ev.Key() {
				case tcell.KeyCtrlL:
					s.Sync()
				default:
					if ev.Rune() == 'Q' || ev.Rune() == 'q' {
						close(quit)
						return
					}
				}
			case *tcell.EventResize:
				s.Sync()
				s.Clear()
				drawPanels(s, sm)
			}
		}
	}()

loop:
	for {
		select {
		case <-quit:
			break loop
		}
	}

	s.Fini()
}
