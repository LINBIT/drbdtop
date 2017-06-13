package display

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"drbdtop.io/drbdtop/pkg/resource"
	"drbdtop.io/drbdtop/pkg/update"
	"github.com/linbit/termui"
)

var drbdtopversion string

/* THINK: movre command* to type */
var commandstr string
var commandFinished bool

type commandMode int

const (
	ex commandMode = iota
	insert
	command
)

type displayMode int

const (
	overview displayMode = iota
	detail
)

type changeIdx int

const (
	up changeIdx = iota
	down
	home
	end
	previous
	next
)

type displayBuffer struct {
	buf  map[string]update.ByRes
	keys []string
	sync.RWMutex
}

var db displayBuffer

type FancyTUI struct {
	resources  *update.ResourceCollection
	lastErr    []error
	cmode      commandMode
	dmode      displayMode
	overview   *overView
	detail     *detailView
	updateDisp chan struct{}
}

func NewFancyTUI(d time.Duration) FancyTUI {
	e := termui.Init()
	if e != nil {
		panic(e)
	}

	db.buf = make(map[string]update.ByRes)
	return FancyTUI{
		resources:  update.NewResourceCollection(d),
		cmode:      ex,
		dmode:      overview,
		overview:   NewOverView(),
		detail:     NewDetailView(),
		updateDisp: make(chan struct{}),
	}
}

func (f *FancyTUI) SetVersion(v string) {
	drbdtopversion = fmt.Sprintf("DRBDTOP %s %s", v, getVersionInfo())
	f.overview.header.Text = drbdtopversion
	f.detail.header.Text = drbdtopversion
}

func (f *FancyTUI) UpdateResources(event <-chan resource.Event, err <-chan error) {
	for {
		select {
		case evt := <-event:
			if evt.Target == resource.EOF {
				// THINK: channel or something...
				// done = true
			}
			f.resources.Update(evt)
			f.updateDisp <- struct{}{}
		case err := <-err:
			if len(f.lastErr) >= 5 {
				f.lastErr = append(f.lastErr[1:], err)
			} else {
				f.lastErr = append(f.lastErr, err)
			}
		}
	}
}

func (f *FancyTUI) UpdateDisp() {
	for {
		<-f.updateDisp
		f.resources.RLock()

		db.Lock()
		if f.dmode == overview && !f.overview.locked { // full update
			db.keys = []string{}
			for _, r := range f.resources.List {
				db.buf[r.Res.Name] = *r
				db.keys = append(db.keys, r.Res.Name)
			}
		} else if f.dmode == overview && f.overview.locked { // update only known keys
			for _, r := range f.resources.List {
				resname := r.Res.Name
				for _, k := range db.keys {
					if k == resname {
						db.buf[resname] = *r
					}
				}
			}
		} else if f.dmode == detail { // update only the currently selected res
			if f.detail.selres != "" {
				for _, r := range f.resources.List {
					resname := r.Res.Name
					if resname == f.detail.selres {
						db.buf[resname] = *r
						break
					}
				}
			}
		}
		db.Unlock()

		if f.dmode == overview {
			f.overview.Update()
		} else if f.dmode == detail {
			f.detail.Update()
		}
		f.resources.RUnlock()
	}
}

func (f *FancyTUI) Display(event <-chan resource.Event, err <-chan error) {
	// done := false
	defer termui.Close()
	f.initHandlers()
	f.overview.UpdateGUI()

	go f.UpdateResources(event, err)
	f.resources.OrderBy(update.Danger, update.Size, update.Name)
	go f.UpdateDisp()

	termui.Loop()
}

func (f *FancyTUI) setLocked() {
	if f.dmode == overview && !f.overview.locked {
		f.overview.SetIdx(home)
		f.overview.SetLocked(true)
		f.overview.ResetIdxHighlight()
	}
}

func (f *FancyTUI) toggleLocked() {
	if f.dmode == overview {
		f.overview.SetIdx(home)
		f.overview.ToggleLocked()
		f.overview.ResetIdxHighlight()
		if f.overview.locked {
			f.overview.SetIdx(home)
		}
	}
}

func (f *FancyTUI) initHandlers() {
	registerDefaultHandler := func(key string, p *termui.Par) {
		termui.Handle("/sys/kbd/"+key, func(e termui.Event) {
			if f.cmode == insert {
				insertMode(e, p)
				return
			}
			if f.dmode == detail {
				f.detail.setWindow(e)
			}
		})
	}

	registerCmdHandler := func(key string, p *termui.Par) {
		termui.Handle("/sys/kbd/"+key, func(e termui.Event) {
			if f.cmode == insert {
				insertMode(e, p)
				return
			}

			if f.dmode == overview && f.overview.locked {
				f.cmode = command
				f.cmdMode(e, p)
			} else if f.dmode == detail {
				f.detail.setWindow(e)
			}
		})
	}
	/* THINK: find a better way */
	defHandlers := "befghilnotuvwxyz" + "BEFGHIJKLMNOQRTUVWXYZ" + "0123456789" + "!§$%&()[],;-.:_+*~<>|"
	for _, h := range defHandlers {
		registerDefaultHandler(string(h), f.overview.footer)
	}

	cmdHandlers := "acdmprs" + "ACDPS"
	for _, h := range cmdHandlers {
		registerCmdHandler(string(h), f.overview.footer)
	}

	/* "special" handlers that override the default behavior; don't forget to rm these from defHandlers */
	termui.Handle("/sys/kbd/q", func(e termui.Event) {
		if f.cmode == insert {
			insertMode(e, f.overview.footer)
			return
		}
		if f.dmode == overview {
			termui.StopLoop()
		} else if f.dmode == detail {
			f.dmode = overview
			f.overview.UpdateGUI()
		}
	})

	/* MOVEMENT */
	kbdDown := func(e termui.Event) {
		if f.cmode == insert {
			insertMode(e, f.overview.footer)
			return
		}

		if f.dmode == overview {
			if !f.overview.locked {
				f.setLocked()
			}
			f.overview.SetIdx(down)
		}
	}

	termui.Handle("/sys/kbd/j", kbdDown)
	termui.Handle("/sys/kbd/<down>", kbdDown)

	kbdUp := func(e termui.Event) {
		if f.cmode == insert {
			insertMode(e, f.overview.footer)
			return
		}

		if f.dmode == overview {
			if !f.overview.locked {
				f.setLocked()
			}
			f.overview.SetIdx(up)
		}
	}

	termui.Handle("/sys/kbd/k", kbdUp)
	termui.Handle("/sys/kbd/<up>", kbdUp)

	termui.Handle("/sys/kbd/<tab>", func(termui.Event) {
		f.cmode = ex
		f.toggleLocked()
	})

	termui.Handle("/sys/kbd/<home>", func(termui.Event) {
		if f.cmode == insert {
			return
		}

		if f.dmode == overview {
			if !f.overview.locked {
				f.setLocked()
			}
			f.overview.SetIdx(home)
		}
	})

	termui.Handle("/sys/kbd/<end>", func(termui.Event) {
		if f.cmode == insert {
			return
		}

		if f.dmode == overview {
			if !f.overview.locked {
				f.setLocked()
			}
			f.overview.SetIdx(end)
		}
	})

	termui.Handle("/sys/kbd/<previous>", func(termui.Event) {
		if f.cmode == insert {
			return
		}

		if f.dmode == overview {
			if !f.overview.locked {
				f.setLocked()
			}
			f.overview.SetIdx(previous)
		}
	})

	termui.Handle("/sys/kbd/<next>", func(termui.Event) {
		if f.cmode == insert {
			return
		}

		if f.dmode == overview {
			if !f.overview.locked {
				f.setLocked()
			}
			f.overview.SetIdx(next)
		}
	})

	/* REST */
	termui.Handle("/sys/kbd//", func(termui.Event) {
		if f.cmode == ex {
			if !f.overview.locked {
				return
			}
			f.overview.footer.Text = "Regex: "
			f.cmode = insert
		}

		termui.Render(f.overview.footer)
	})

	termui.Handle("/sys/kbd/<backspace>", func(termui.Event) {
		if f.cmode == insert {
			// TODO: make this more clever
			if len(f.overview.footer.Text) > len("Regex: ") {
				f.overview.footer.Text = f.overview.footer.Text[:len(f.overview.footer.Text)-1]
			}
		}

		termui.Render(f.overview.footer)
	})

	termui.Handle("/sys/kbd/<escape>", func(termui.Event) {
		f.reset()
	})

	termui.Handle("/sys/kbd/<enter>", func(termui.Event) {
		if f.dmode == overview {
			if f.cmode == insert {
				defer func() {
					f.overview.setLockedStr()
					f.overview.UpdateTable()
					// termui.Render(f.overview.footer)
					f.cmode = ex
				}()

				sa := strings.SplitN(f.overview.footer.Text, ":", 2)
				s := sa[1]
				if len(s) > 1 { /* contains the space */
					s = s[1:]
				} else {
					return
				}

				rgx, err := regexp.Compile(s)
				if err != nil {
					return
				}

				for idx, e := range f.overview.tbl.Rows[1:] {
					if rgx.MatchString(e[0]) {
						f.overview.ResetIdxHighlight()
						f.overview.selidx = idx
						break
					}
				}
			} else if f.cmode == ex {
				if f.overview.selres != "" {
					f.detail.selres = f.overview.selres
					f.dmode = detail
					f.detail.UpdateGUI()
				}
			}
		}
	})

	termui.Handle("/sys/wnd/resize", func(e termui.Event) {
		if f.dmode == overview {
			f.overview.tblheight = termui.TermHeight() - f.overview.header.Height - f.overview.footer.Height
			f.overview.tblwidth = termui.TermWidth()
			f.overview.SetTableElems()
			f.overview.tbl.Width = f.overview.tblwidth
			f.overview.tbl.Height = f.overview.tblheight
			f.overview.UpdateGUI()
		}
	})
}

func insertMode(e termui.Event, p *termui.Par) {
	k, _ := e.Data.(termui.EvtKbd)
	p.Text += k.KeyStr
	termui.Render(p)
}

func (f *FancyTUI) reset() {
	f.cmode = ex
	commandstr = ""
	commandFinished = false
	if f.dmode == overview {
		f.overview.setLockedStr()
	} else if f.dmode == detail {
		f.dmode = overview
		f.overview.UpdateGUI()
	}
}

func (f *FancyTUI) cmdMode(e termui.Event, p *termui.Par) {
	k, _ := e.Data.(termui.EvtKbd)
	keyStr := k.KeyStr

	valid := true
	switch keyStr {
	case "d":
		switch commandstr {
		case "": // disk menu
			p.Text = "a/d: attach/detach selected | A/D: attach/detach all"
		default:
			commandFinished = true
		}
	case "a":
		switch commandstr {
		case "": // adjust menu
			p.Text = "a: adjust selected | A: adjust all"
		default:
			commandFinished = true
		}
	case "c":
		switch commandstr {
		case "": // connection menu
			p.Text = "c/d/m: connect/disconnect/discard my data selected | C/D: connect/disconnect all"
		case "c":
			commandFinished = true
		}
	case "r":
		switch commandstr {
		case "": // role menu
			p.Text = "p/s: primary/secondary selected | P/S: primary/secondary all"
		}
	case "A", "C", "D", "P", "S", "p", "s", "m":
		commandFinished = true
	default:
		valid = false
	}

	if valid {
		commandstr += keyStr
	}

	p.Text += " | <esc>: Abort command"

	if commandFinished {
		var finalCmd string
		switch commandstr {
		/* adjust */
		case "aa":
			finalCmd = "drbdadm adjust " + f.overview.selres
		case "aA":
			finalCmd = "drbdadm adjust all"
		/* disk */
		case "da":
			finalCmd = "drbdadm attach " + f.overview.selres
		case "dA":
			finalCmd = "drbdadm attach all"
		case "dd":
			finalCmd = "drbdadm detach " + f.overview.selres
		case "dD":
			finalCmd = "drbdadm detach all"
		/* connection */
		case "cc":
			finalCmd = "drbdadm connect " + f.overview.selres
		case "cC":
			finalCmd = "drbdadm connect all"
		case "cd":
			finalCmd = "drbdadm disconnect " + f.overview.selres
		case "cD":
			finalCmd = "drbdadm disconnect all"
		case "cm":
			finalCmd = "drbdadm connect --discard-my-data " + f.overview.selres
		/* role */
		case "rp":
			finalCmd = "drbdadm primary " + f.overview.selres
		case "rs":
			finalCmd = "drbdadm secondary " + f.overview.selres
		case "rP":
			finalCmd = "drbdadm primary all"
		case "rS":
			finalCmd = "drbdadm secondary all"
		}

		cmdOK := false
		if finalCmd != "" {
			p.Text = fmt.Sprintf("Executing '%s'... ", finalCmd)
			termui.Render(p)
			args := strings.Split(finalCmd, " ")
			if len(args) > 1 {
				cmd := exec.Command(args[0], args[1:]...)
				cmd.Start()
				if err := cmd.Wait(); err != nil {
					p.Text += fmt.Sprintf("%v", err)
				} else {
					p.Text += setOK()
					cmdOK = true
				}
			} else {
				p.Text = "Aborting: Valid command, but too few arguments?!"
			}
		} else {
			p.Text = "Aborting: Your input was not a valid command!"
		}

		// hard sleep here, IMO it makes more sense than tmpFooterMsg()
		termui.Render(p)

		sl := 4 * time.Second // give user to read the error message
		if cmdOK {
			sl = 2 * time.Second // user is happy, make time shorter
		}
		time.Sleep(sl)

		f.toggleLocked()
		f.reset()
	}
	termui.Render(p)
}

func tmpFooterMsg(f *termui.Par, t string, d time.Duration) {
	old := f.Text
	go func() {
		f.Text = t
		termui.Render(f)
		time.Sleep(d)
		f.Text = old
		termui.Render(f)
	}()
}

func switchDisp(grid *termui.Grid) {
	termui.Body = grid
	termui.Body.Width = termui.TermWidth()
	termui.Body.Align()
	termui.Clear()
	termui.Render(termui.Body)
}

func setFail(danger uint64) string {
	s := "[" + "✗ "
	if danger != 0 {
		s += "(" + strconv.Itoa(int(danger)) + ")"
	}
	s += "](fg-red)"
	return s
}

func setOK() string {
	return colGreen("✓ ", false)
}

func setColor(s, name string, bold bool) string {
	c := "(fg-" + name
	if bold {
		c += ",fg-bold"
	}
	c += ")"
	return "[" + s + "]" + c

}

func colRed(s string, bold bool) string     { return setColor(s, "red", bold) }
func colGreen(s string, bold bool) string   { return setColor(s, "green", bold) }
func colBlue(s string, bold bool) string    { return setColor(s, "blue", bold) }
func colDefault(s string, bold bool) string { return setColor(s, "default", bold) }