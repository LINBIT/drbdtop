/*
 *drbdtop - statistics for DRBD
 *Copyright © 2017 Hayley Swimelar and Roland Kammerer
 *
 *This program is free software; you can redistribute it and/or modify
 *it under the terms of the GNU General Public License as published by
 *the Free Software Foundation; either version 2 of the License, or
 *(at your option) any later version.
 *
 *This program is distributed in the hope that it will be useful,
 *but WITHOUT ANY WARRANTY; without even the implied warranty of
 *MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *GNU General Public License for more details.
 *
 *You should have received a copy of the GNU General Public License
 *along with this program; if not, see <http://www.gnu.org/licenses/>.
 */

package display

import "github.com/linbit/termui"

var lockedHelp string = "q: QUIT | j/k: down/up | /: find | r: role | a: adjust | d: disk | c: connection | <tab>: Toggle updates"
var unlockedHelp string = "q: QUIT | j/k: down/up | <tab>: Toggle updates"

func window(selidx, maxItems, overall int) (from, to int) {
	block := 0
	if selidx > 0 && maxItems > 0 {
		block = selidx / maxItems
	}
	from = block * maxItems
	to = from + maxItems
	if to > overall {
		to = overall
	}

	if from < 0 || to < 0 {
		from, to = 0, 0
	}
	return from, to
}

type overView struct {
	grid                *termui.Grid
	tbl                 *termui.Table
	tblheight, tblwidth int
	tblelems            int
	header, footer      *termui.Par
	selidx              int
	selres              string
	from, to            int
	locked              bool // TODO maybe make this a propper lock
}

func NewOverView() *overView {
	o := overView{
		grid:   nil,
		from:   -1,
		to:     -1,
		locked: false,
	}

	o.header = termui.NewPar(drbdtopversion)
	o.header.Height = 1
	o.header.TextFgColor = termui.ColorWhite
	o.header.Border = false

	o.footer = termui.NewPar(unlockedHelp)
	o.footer.Height = 1
	o.footer.TextFgColor = termui.ColorWhite
	o.footer.Border = false

	o.tblheight = termui.TermHeight() - o.header.Height - o.footer.Height
	o.tblwidth = termui.TermWidth()
	o.SetTableElems()

	tblrows := [][]string{{"Name", "Role", "Disks", "Connections", "Overall"}}

	table := termui.NewTable()
	table.Rows = tblrows
	table.FgColor = termui.ColorWhite
	table.BgColor = termui.ColorDefault
	table.TextAlign = termui.AlignLeft
	table.Separator = false
	table.Border = true

	table.Analysis()
	table.SetSize()

	o.tbl = table
	o.tbl.FgColors[0] |= termui.AttrBold
	o.tbl.Height = o.tblheight
	o.tbl.Width = o.tblwidth
	o.setLockedStr()

	return &o
}

func (o *overView) SetTableElems() {
	o.tblelems = o.tblheight - 3
}

func (o *overView) useCache(from, to int) bool {
	if o.from == from && o.to == to {
		return true
	}
	return false
}

func dangerToString(danger uint64) string {
	if danger == 0 {
		return setOK()
	}
	return setFail(danger)
}

func (o *overView) UpdateTable() {
	db.RLock()
	defer db.RUnlock()
	from, to := window(o.selidx, o.tblelems, len(db.keys))

	if !o.useCache(from, to) || !o.locked {
		o.from, o.to = from, to
		if len(db.keys[from:to]) > 0 {
			tblrows := make([][]string, len(db.keys[from:to])+1)
			tblrows[0] = []string{"Name", "Role", "Disks", "Peer Disks", "Connections", "Overall"}
			for idx, rname := range db.keys[from:to] {
				r := db.buf[rname]
				res := db.buf[rname].Res
				devdanger := r.Device.Danger

				var conndanger uint64
				for _, c := range r.Connections {
					conndanger += c.Danger
				}

				var pddanger uint64
				for _, pd := range r.PeerDevices {
					pddanger += pd.Danger
				}

				role := res.Role
				if role == "Primary" {
					role = "[" + role + "](fg-green)"
				}

				tblrows[idx+1] = []string{res.Name, role,
					dangerToString(devdanger), dangerToString(pddanger), dangerToString(conndanger), dangerToString(r.Danger)}
			}
			o.tbl.SetRows(tblrows)
		}
	}

	if o.locked {
		s := o.selidx % o.tblelems
		if len(o.tbl.Rows) > s+1 {
			o.tbl.BgColors[s+1] = termui.ColorBlue
			o.selres = o.tbl.Rows[s+1][0]
		} else {
			o.selres = ""
		}
	}
}

func (o *overView) UpdateGUI() {
	o.UpdateTable()

	o.grid = termui.NewGrid()

	o.grid.AddRows(
		termui.NewRow(
			termui.NewCol(12, 0, o.header)),
		termui.NewRow(
			termui.NewCol(12, 0, o.tbl)),
		termui.NewRow(
			termui.NewCol(12, 0, o.footer)))

	switchDisp(o.grid)
}

func (o *overView) Update() {
	o.UpdateTable()
	termui.Render(o.tbl, o.footer)
}

func (o *overView) ResetIdxHighlight() {
	/* reset old selidx */
	tblheight := termui.TermHeight() - o.header.Height - o.footer.Height
	s := o.selidx % (tblheight - 3)
	if len(o.tbl.Rows) > s+1 {
		o.tbl.BgColors[s+1] = termui.ColorDefault
	}
	o.selres = ""
}

func (o *overView) SetIdx(c changeIdx) {
	if !o.locked {
		return
	}

	o.ResetIdxHighlight()

	switch c {
	case down:
		o.selidx++
		o.selidx %= len(db.keys)
	case up:
		if o.selidx == 0 {
			o.selidx = len(db.keys) - 1
		} else {
			o.selidx--
		}
	case home:
		o.selidx = 0
	case end:
		o.selidx = len(db.keys) - 1
	case previous:
		newidx := o.selidx - (o.tbl.Height - 3)
		if newidx >= 0 {
			o.selidx = newidx
		}
	case next:
		newidx := o.selidx + (o.tbl.Height - 3)
		if newidx <= len(db.keys)-1 {
			o.selidx = newidx
		}
	}

	o.UpdateTable()
	termui.Render(o.tbl, o.footer)
}

func (o *overView) SetLocked(l bool) {
	o.locked = l
	o.setLockedStr()
}

func (o *overView) ToggleLocked() {
	o.SetLocked(!o.locked)
}

func (o *overView) setLockedStr() {
	s := "◉"
	if o.locked {
		o.tbl.BorderLabel = "[" + s + " (FROZEN)](fg-red)" + " Resource List"
		o.footer.Text = lockedHelp
	} else {
		o.tbl.BorderLabel = s + " (LIVE UPDATING)" + " Resource List"
		o.footer.Text = unlockedHelp
	}
	termui.Render(o.tbl, o.footer)
}
