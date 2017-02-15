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

const timeFormat = "2006-01-02T15:04:05.000000-0700"

var resourceFieldKeys = []string{"name", "role", "suspended", "write-ordering"}
var connectionFieldKeys = []string{"name", "peer-node-id", "conn-name", "connection", "role", "congested"}
var deviceFieldKeys = []string{"name", "volume", "minor", "disk", "size", "read", "written", "al-writes", "bm-writes", "upper-pending", "lower-pending", "al-suspended", "blocked"}
var peerDeviceFieldKeys = []string{"name", "peer-node-id", "conn-name", "volume", "replication", "peer-disk", "resync-suspended", "received", "sent", "out-of-sync", "pending", "unacked"}

type Event struct {
	timeStamp time.Time
	target    string
	fields    map[string]string
}

type Status struct {
	Name          string
	Role          string
	Suspended     string
	WriteOrdering string
	Connections   map[string]Connection
	Devices       map[string]Device
	PeerDevices   map[string]PeerDevice
	StartTime     time.Time
	CurrentTime   time.Time

	// Calulated Values
	uptime      time.Time
	updateCount int

	numPeerDevs    int
	numDevs        int
	numConnections int
}

type Connection struct {
	peerNodeID       string
	connectionName   string
	connectionStatus string
	role             string
	congested        string
	startTime        time.Time
	currentTime      time.Time
	// Calculated Values

	uptime      time.Time
	updateCount int
}

type Device struct {
	volumes map[string]DevVolume
}

type DevVolume struct {
	minor                string
	diskState            string
	size                 int
	readKiB              int
	writtenKiB           int
	activityLogUpdates   int
	bitMapUpdates        int
	upperPending         int
	lowerPending         int
	activityLogSuspended string
	blocked              string
	startTime            time.Time
	currentTime          time.Time

	// Calculated Values
	uptime      time.Time
	updateCount int

	initialReadKiB   int
	totalReadKiB     int
	readKiBPerSecond float32

	initialActivityLogUpdates int
	totalActivityLogUpdates   int
	alUpdatesPerSecond        float32

	initialBitMapUpdates   int
	totalBitMapUpdates     int
	bitMapUpdatesPerSecond float32

	maxUpperPending   int
	minUpperPending   int
	avgUpperPending   float32
	totalUpperPending int

	maxLowerPending   int
	minLowerPending   int
	avgLowerPending   float32
	totalLowerPending int
}

type PeerDevice struct {
	peerNodeID     string
	connectionName string
	volumes        map[string]PeerDevVol
}

type PeerDevVol struct {
	replicationStatus string
	diskState         string
	resyncSuspended   string
	receivedKiB       int
	sentKiB           int
	outOfSyncKiB      int
	pendingWrites     int
	unackedWrites     int
	startTime         time.Time
	currentTime       time.Time

	// Calulated Values
	uptime      time.Time
	updateCount int

	maxOutOfSyncKiB   int
	minOutOfSyncKiB   int
	avgOutOfSyncKiB   float32
	totalOutOfSyncKiB int

	maxPendingWrites   int
	minPendingWrites   int
	avgPendingWrites   float32
	totalPendingWrites int

	maxUnackedWrites   int
	minUnackedWrites   int
	avgUnackedWrites   float32
	totalUnackedWrites int

	initialReceivedKiB int
	totalReceivedKiB   int
	receivedKiBSecond  float32

	initialSentKiB int
	totalSentKiB   int
	sentKiBSecond  float32
}
