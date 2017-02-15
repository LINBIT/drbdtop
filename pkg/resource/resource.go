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
