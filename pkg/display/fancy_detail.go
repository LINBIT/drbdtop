package display

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"drbdtop.io/drbdtop/pkg/convert"
	"drbdtop.io/drbdtop/pkg/resource"
	"drbdtop.io/drbdtop/pkg/update"

	"github.com/linbit/termui"
)

type win int

const (
	oos win = iota
	status
	detailedstatus
)

type uiGauge struct {
	p *termui.Par
	g *termui.Gauge
}

type detailView struct {
	grid           *termui.Grid
	header, footer *termui.Par
	oldselres      string
	selres         string
	volGauges      map[string]uiGauge // oos view
	status         *termui.Par        // status view
	window         win
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

	d.status = termui.NewPar("")
	d.status.Height = 3
	d.status.TextFgColor = termui.ColorWhite

	d.footer = termui.NewPar("q: overview | o: oos | s: status | d: detailed status")
	d.footer.Height = 1
	d.footer.TextFgColor = termui.ColorWhite
	d.footer.Border = false
	d.window = status

	return &d
}

func (d *detailView) printRes(r *update.ByRes) {
	d.status.Text += fmt.Sprintf("%s: %s: (%d) ", colDefault("Resource", true), r.Res.Name, r.Danger)

	if r.Res.Suspended != "no" {
		d.status.Text += fmt.Sprintf("(Suspended)")
	}

	d.status.Text += fmt.Sprintf("\n")
}

func (dv *detailView) printLocalDisk(r *update.ByRes) {
	st := dv.status.Text

	st += fmt.Sprintf(" %s(%s):\n", colDefault("Local Disc", true), r.Res.Role)

	d := r.Device

	var keys []string
	for k := range d.Volumes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := d.Volumes[k]
		st += fmt.Sprintf("  volume %s (/dev/drbd%s):", k, v.Minor)
		dState := v.DiskState

		if dState != "UpToDate" {
			st += fmt.Sprintf(" %s", dState)

			st += fmt.Sprintf("(%s)", v.DiskHint)
		}

		if dv.window == detailedstatus {
			if v.Blocked != "no" {
				st += fmt.Sprintf(" Blocked: %s ", d.Volumes[k].Blocked)
			}

			if v.ActivityLogSuspended != "no" {
				st += fmt.Sprintf(" Activity Log Suspended: %s ", d.Volumes[k].Blocked)
			}

			st += fmt.Sprintf("\n")
			st += fmt.Sprintf("    size: %s total-read:%s read/Sec:%s total-written:%s written/Sec:%s ",
				convert.KiB2Human(float64(v.Size)),
				convert.KiB2Human(float64(v.ReadKiB.Total)), convert.KiB2Human(v.ReadKiB.PerSecond),
				convert.KiB2Human(float64(v.WrittenKiB.Total)), convert.KiB2Human(v.WrittenKiB.PerSecond))
		}
		st += fmt.Sprintf("\n")
	}
	dv.status.Text = st
}

func (d *detailView) printConn(c *resource.Connection) {
	st := d.status.Text
	st += fmt.Sprintf("%s", colDefault(fmt.Sprintf(" Connection to %s", c.ConnectionName), true))

	st += fmt.Sprintf("(%s):", c.Role)

	if c.ConnectionStatus != "Connected" {
		st += fmt.Sprintf("%s", c.ConnectionStatus)
		st += fmt.Sprintf("(%s)", c.ConnectionHint)
	}

	if c.Congested != "no" {
		st += fmt.Sprintf(" Congested ")
	}

	st += fmt.Sprintf("\n")

	d.status.Text = st
}

func (dv *detailView) printPeerDev(r *update.ByRes, conn string) {
	st := dv.status.Text
	d := r.PeerDevices[conn]

	var keys []string
	for k := range d.Volumes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := d.Volumes[k]
		st += fmt.Sprintf("  volume %s: ", k)

		if v.ResyncSuspended != "no" {
			st += fmt.Sprintf(" ResyncSuspended:%s ", v.ResyncSuspended)
		}
		st += fmt.Sprintf("\n")

		if v.ReplicationStatus != "Established" {
			st += fmt.Sprintf("   Replication:%s", v.ReplicationStatus)
			st += fmt.Sprintf("(%s)", v.ReplicationHint)
		}

		if strings.HasPrefix(v.ReplicationStatus, "Sync") {
			st += fmt.Sprintf(" %.1f%% remaining",
				(float64(v.OutOfSyncKiB.Current)/float64(r.Device.Volumes[k].Size))*100)
		}

		if v.DiskState != "UpToDate" {
			st += fmt.Sprintf("\n   %s", v.DiskState)
			st += fmt.Sprintf("(%s)", v.DiskHint)
		}

		st += fmt.Sprintf("\n")

		if dv.window == detailedstatus {
			st += fmt.Sprintf("   Sent: total:%s Per/Sec:%s\n",
				convert.KiB2Human(float64(v.SentKiB.Total)), convert.KiB2Human(v.SentKiB.PerSecond))

			st += fmt.Sprintf("   Received: total:%s Per/Sec:%s\n",
				convert.KiB2Human(float64(v.ReceivedKiB.Total)), convert.KiB2Human(v.ReceivedKiB.PerSecond))

			st += fmt.Sprintf("   OutOfSync: current:%s average:%s min:%s max:%s\n",
				convert.KiB2Human(float64(v.OutOfSyncKiB.Current)),
				convert.KiB2Human(v.OutOfSyncKiB.Avg),
				convert.KiB2Human(float64(v.OutOfSyncKiB.Min)),
				convert.KiB2Human(float64(v.OutOfSyncKiB.Max)))

			st += fmt.Sprintf("   PendingWrites: current:%s average:%s min:%s max:%s\n",
				fmt.Sprintf("%.1f", v.PendingWrites.Current),
				fmt.Sprintf("%.1f", v.PendingWrites.Avg),
				fmt.Sprintf("%.1f", v.PendingWrites.Min),
				fmt.Sprintf("%.1f", v.PendingWrites.Max))

			st += fmt.Sprintf("   UnackedWrites: current:%s average:%s min:%s max:%s\n",
				fmt.Sprintf("%.1f", v.UnackedWrites.Current),
				fmt.Sprintf("%.1f", v.UnackedWrites.Avg),
				fmt.Sprintf("%.1f", v.UnackedWrites.Min),
				fmt.Sprintf("%.1f", v.UnackedWrites.Max))

			st += fmt.Sprintf("\n")
		}
	}
	dv.status.Text = st
}

func (d *detailView) printByRes(r *update.ByRes) {

	d.status.Text = ""
	d.printRes(r)

	d.printLocalDisk(r)
	d.status.Text += fmt.Sprintf("\n")

	var connKeys []string
	for j := range r.Connections {
		connKeys = append(connKeys, j)
	}
	sort.Strings(connKeys)

	for _, conn := range connKeys {
		if c, ok := r.Connections[conn]; ok {
			d.printConn(c)

			if _, ok := r.PeerDevices[conn]; ok {
				d.printPeerDev(r, conn)
			}
			d.status.Text += fmt.Sprintf("\n")
		}
	}
}

func (d *detailView) UpdateStatus() {
	db.RLock()
	defer db.RUnlock()
	for _, rname := range db.keys {
		if rname == d.selres {
			res := db.buf[rname]
			d.printByRes(&res)
		}
	}
	// d.status.Text = "rck\nrck2\nrck3"

	d.oldselres = d.selres
}

func (d *detailView) UpdateOOS() {
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

					var oos, nrPeerDevs uint64
					for _, pdev := range res.PeerDevices {
						pvol := pdev.Volumes[k]
						oos += pvol.OutOfSyncKiB.Current
						nrPeerDevs++
					}

					// oosp is oos over *all* peers, sizes are (roughly) the same, so multiply v.Size by nrPeerDevs, to get sane percentage
					oosp := int(float64(oos*100) / float64(v.Size*nrPeerDevs))
					if oosp == 0 && oos > 0 {
						oosp = 1 // make it visable that something is oos
					}
					g.Percent = oosp

					ps := fmt.Sprintf("Vol %s (/dev/drbd%s)", k, v.Minor)
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

func (d *detailView) updateContent() {
	switch d.window {
	case oos:
		d.UpdateOOS()
	case status, detailedstatus:
		d.UpdateStatus()
	default:
		panic("window")
	}

}

func (d *detailView) updateGUI(updateContent bool) {
	d.header.Text = drbdtopversion + " - Detail View for " + d.selres
	if updateContent {
		d.updateContent()
	}
	d.grid = termui.NewGrid()
	d.grid.AddRows(
		termui.NewRow(
			termui.NewCol(12, 0, d.header)))

	var heights int

	switch d.window {
	case oos:
		for _, uig := range d.volGauges {
			d.grid.AddRows(
				termui.NewRow(
					termui.NewCol(3, 0, uig.p),
					termui.NewCol(9, 0, uig.g)))
		}
		heights = len(d.volGauges)*3 + d.header.Height + d.footer.Height
	case status, detailedstatus:
		statusheight := termui.TermHeight() - d.header.Height - d.footer.Height
		d.status.Height = statusheight
		d.grid.AddRows(
			termui.NewRow(
				termui.NewCol(12, 0, d.status)))
		heights = d.status.Height + d.header.Height + d.footer.Height
	default:
		panic("window")
	}

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

// The public one, where we really want to update everything
func (d *detailView) UpdateGUI() {
	d.updateGUI(true)
}

func (d *detailView) setWindow(e termui.Event) {
	k, _ := e.Data.(termui.EvtKbd)
	old := d.window
	switch k.KeyStr {
	case "o":
		d.window = oos
	case "s":
		d.window = status
	case "d":
		d.window = detailedstatus
	}

	if old != d.window {
		d.updateGUI(true)
	}
}

func (d *detailView) Update() {
	// TODO: move that switch bodies to functions
	switch d.window {
	case oos:
		was := len(d.volGauges)
		d.UpdateOOS()
		is := len(d.volGauges)
		if was != is {
			d.updateGUI(false)
		} else {
			var keys []string
			for k := range d.volGauges {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				uig := d.volGauges[k]
				termui.Render(uig.p, uig.g)
			}
		}
	case status, detailedstatus:
		d.UpdateStatus()
		termui.Render(d.status)
	default:
		panic("window")
	}
}