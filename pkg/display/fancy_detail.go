package display

import (
	"fmt"
	"os"

	"drbdtop.io/drbdtop/pkg/resource"

	"github.com/linbit/termui"
)

type uiGauge struct {
	p   *termui.Par
	g   *termui.Gauge
	vol resource.DevVolume
}

type detailView struct {
	grid           *termui.Grid
	header, footer *termui.Par
	oldselres      string
	selres         string
	volGauges      map[string]uiGauge
}

func NewDetailView() *detailView {
	d := detailView{
		grid:      nil,
		volGauges: make(map[string]uiGauge),
	}

	d.header = termui.NewPar("")
	d.header.Height = 1
	d.header.TextFgColor = termui.ColorWhite
	d.header.Border = false

	d.footer = termui.NewPar("q: back | allthefancything")
	d.footer.Height = 1
	d.footer.TextFgColor = termui.ColorWhite
	d.footer.Border = false

	return &d
}

var fakepercent int = 0

func (d *detailView) UpdateGauges() {
	d.header.Text = drbdtopversion + " - Detail View for " + d.selres

	db.RLock()
	defer db.RUnlock()

	for _, rname := range db.keys {
		if rname == d.selres {
			res := db.buf[rname]
			dev := res.Device
			vols := dev.Volumes
			if d.selres != d.oldselres {
				/* THINK or empty the old one? */
				d.volGauges = make(map[string]uiGauge)
			}
			for k, v := range vols {
				if _, ok := d.volGauges[k]; !ok {
					g := termui.NewGauge()
					g.Height = 3
					g.BorderLabel = "Out of sync"
					g.BorderLabelFg = termui.ColorRed
					g.Percent = fakepercent

					ps := fmt.Sprintf("Volume: %s (/dev/drbd/%s)", k, v.Minor)
					p := termui.NewPar(ps)
					p.Height = 3

					e := d.volGauges[k]
					e.g, e.p = g, p
					d.volGauges[k] = e
				}
			}
			break
		}
	}
	d.oldselres = d.selres
}

func (d *detailView) UpdateGUI() {
	d.UpdateGauges()

	d.grid = termui.NewGrid()
	d.grid.AddRows(
		termui.NewRow(
			termui.NewCol(12, 0, d.header)))

	for _, uig := range d.volGauges {
		d.grid.AddRows(
			termui.NewRow(
				termui.NewCol(6, 0, uig.p),
				termui.NewCol(6, 0, uig.g)))
	}

	heights := len(d.volGauges)*3 + d.header.Height + d.footer.Height
	spacerheight := termui.TermHeight() - heights
	if spacerheight > 0 {
		s := termui.NewPar("")
		s.Border = false
		s.Height = termui.TermHeight() - heights
		fmt.Fprintln(os.Stderr, s.Height)

		d.grid.AddRows(
			termui.NewRow(
				termui.NewCol(12, 0, s)))
	}

	d.grid.AddRows(
		termui.NewRow(
			termui.NewCol(12, 0, d.footer)))

	switchDisp(d.grid)
}

func (d *detailView) Update() {
	was := len(d.volGauges)
	d.UpdateGauges()
	is := len(d.volGauges)
	if was != is {
		d.UpdateGUI()
	} else {
		for _, uig := range d.volGauges {
			fakepercent++
			uig.g.Percent = fakepercent
			fakepercent %= 101
			termui.Render(uig.p, uig.g)
		}
	}

}
