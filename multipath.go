// Package multipath provides a simple way to bond multiple network paths
// between a pair of hosts to form a single connection from the upper layer
// perspective, for throughput and resilience.
//
// The term connection, path and subflow used here is the same as mentioned in
// MP-TCP https://www.rfc-editor.org/rfc/rfc8684.html#name-terminology
//
// Each subflow is a bidirectional byte stream each side in the following form
// until being disrupted or the connection ends. When establishing the very
// first subflow, the client sends an all-zero connnection ID (CID) and the
// server sends the assigned CID back. Subsequent subflows use the same CID.
//
//       ----------------------------------------------------
//      |  version(1)  |  cid(8)  |  frames (...)  |
//       ----------------------------------------------------
//
// There are two types of frames. Data frame carries application data while ack
// frame carries acknowledgement to the frame just received. When one data
// frame is not acked in time, it is sent over another subflow, until all
// available subflows have been tried. Payload size and frame number uses
// variable-length integer encoding as described here: https:
// //tools.ietf.org/html/draft-ietf-quic-transport-29#section-16
//
//       --------------------------------------------------------
//      |  payload size(1-8)  |  frame number (1-8)  |  payload  |
//       --------------------------------------------------------
//
//       ---------------------------------------
//      |  00000000  |  ack frame number (1-8)  |
//       ---------------------------------------
//
// Ack frames with frame number < 10 are reserved for control. For now only 0
// and 1 are used, for ping and pong frame respectively. They are for updating
// RTT on inactive subflows and detecting recovered subflows.
//
// Ping frame:
//       -------------------------
//      |  00000000  |  00000000  |
//       -------------------------
//
// Pong frame:
//       -------------------------
//      |  00000000  |  00000001  |
//       -------------------------
//

package multipath

import (
	"errors"
	"time"

	"github.com/getlantern/golog"
)

const (
	minFrameNumber uint64 = 10
	frameTypePing  uint64 = 0
	frameTypePong  uint64 = 1

	maxFrameSizeToCalculateRTT int = 1500
	recieveQueueLength             = 4096
	probeInterval                  = time.Minute
)

var (
	ErrUnexpectedVersion = errors.New("unexpected version")
	ErrUnexpectedCID     = errors.New("unexpected connnection ID")
	ErrClosed            = errors.New("closed connection")
	log                  = golog.LoggerFor("multipath")
)

type frame struct {
	fn    uint64
	bytes []byte
}

type sendFrame struct {
	fn              uint64
	sz              uint64
	buf             []byte
	retransmissions int
}

func (f *sendFrame) isDataFrame() bool {
	return f.sz > 0
}

type statsTracker interface {
	onRecv(uint64)
	onSent(uint64)
	onRetransmit(uint64)
	updateRTT(time.Duration)
}

type nullStatsTracker struct{}

func (t nullStatsTracker) onRecv(uint64)           {}
func (t nullStatsTracker) onSent(uint64)           {}
func (t nullStatsTracker) onRetransmit(uint64)     {}
func (t nullStatsTracker) updateRTT(time.Duration) {}
