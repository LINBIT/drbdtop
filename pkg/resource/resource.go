/*
 *drbdtop - continously update stats on drbd
 *Copyright © 2017 Hayley Swimelar
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

package resource

import (
	"time"
)

type Resource struct {
	Name          string
	Role          string
	Suspended     string
	WriteOrdering string
	Connections   map[string]Connection
	Devices       map[string]Device
	PeerDevices   map[string]PeerDevice
	StartTime     time.Time
	CurrentTime   time.Time
}

type Connection struct {
	peerNodeID       string
	connectionName   string
	connectionStatus string
	role             string
	congested        string
	startTime        time.Time
	currentTime      time.Time
}

type Device struct {
	volume               string
	minor                string
	diskState            string
	size                 int
	readKiB              int
	writtenKiB           int
	activityLogWritesKiB int
	bmWritesKib          int
	upperPending         int
	lowerPending         int
	activityLogSuspended string
	blocked              string
	startTime            time.Time
	currentTime          time.Time
}

type PeerDevice struct {
	peerNodeID        string
	connectionName    string
	volume            string
	replicationStatus string
	diskState         string
	resyncSuspended   string
	received          int //received what?
	sent              int // sent what?
	outOfSyncBlocks   int
	pendingWrites     int
	unackedWrites     int
	startTime         time.Time
	currentTime       time.Time
}
