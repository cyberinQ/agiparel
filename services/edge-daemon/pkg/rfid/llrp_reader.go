// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// Parse real LLRP 1.x tag reports; never infer an EPC, antenna, RSSI or reader clock.
package rfid

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const (
	llrpHeaderSize     = 10
	llrpMaxFrame       = 1 << 20
	llrpROAccessReport = 61
	llrpKeepalive      = 62
	llrpKeepaliveAck   = 72
	llrpTagReport      = 240
	llrpEPCData        = 241
	llrpEPC96          = 13
)

// Missing optional LLRP fields stay unset, not fabricated by the adapter.
// This event is an untrusted reader observation, NOT a gateway-signed production scan.
type RFIDEvent struct {
	TagUID            string     `json:"tag_uid"`
	EPCBits           int        `json:"epc_bits"`
	AntennaID         int        `json:"antenna_id,omitempty"`
	AntennaMeasured   bool       `json:"antenna_measured"`
	RSSI              float64    `json:"rssi,omitempty"`
	RSSIMeasured      bool       `json:"rssi_measured"`
	Timestamp         *time.Time `json:"timestamp,omitempty"`
	ReaderUTCMeasured bool       `json:"reader_utc_measured"`
}

type LLRPHealth struct {
	Connected               bool      `json:"connected"`
	Connects                uint64    `json:"connects"`
	Disconnects             uint64    `json:"disconnects"`
	RejectedFrames          uint64    `json:"rejected_frames"`
	ObservedTags            uint64    `json:"observed_tags"`
	LastConnectedAt         time.Time `json:"last_connected_at,omitempty"`
	LastKeepaliveReceivedAt time.Time `json:"last_keepalive_received_at,omitempty"`
	LastReportReceivedAt    time.Time `json:"last_report_received_at,omitempty"`
	LastError               string    `json:"last_error,omitempty"`
}

type LLRPReader struct {
	Address     string
	Timeout     time.Duration
	IdleTimeout time.Duration
	RetryMin    time.Duration
	RetryMax    time.Duration
	mu          sync.Mutex
	health      LLRPHealth
}

func NewLLRPReader(address string) *LLRPReader {
	return &LLRPReader{Address: address, Timeout: 5 * time.Second, IdleTimeout: 120 * time.Second, RetryMin: 250 * time.Millisecond, RetryMax: 10 * time.Second}
}

func (r *LLRPReader) Health() LLRPHealth {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.health
}

func (r *LLRPReader) updateHealth(fn func(*LLRPHealth)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fn(&r.health)
}

// Listen reconnects with bounded backoff. It only exits on cancellation or invalid
// configuration. Reconnection never replays, signs, or grants custody for events.
func (r *LLRPReader) Listen(ctx context.Context, eventStream chan<- RFIDEvent) error {
	if r == nil || r.Address == "" || eventStream == nil {
		return errors.New("ERR_LLRP_CONFIGURATION")
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	retryMin, retryMax := r.RetryMin, r.RetryMax
	if retryMin <= 0 {
		retryMin = 250 * time.Millisecond
	}
	if retryMax < retryMin {
		retryMax = retryMin
	}
	idleTimeout := r.IdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = 120 * time.Second
	}
	backoff := retryMin
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", r.Address)
		if err == nil {
			r.updateHealth(func(h *LLRPHealth) {
				h.Connected = true
				h.Connects++
				h.LastConnectedAt = time.Now().UTC()
				h.LastError = ""
			})
			// Close the connection as soon as ctx is cancelled, even mid-frame.
			closed := make(chan struct{})
			go func() {
				select {
				case <-ctx.Done():
					_ = conn.Close()
				case <-closed:
				}
			}()
			err = r.readConnection(ctx, conn, eventStream, timeout, idleTimeout)
			close(closed)
			_ = conn.Close()
			r.updateHealth(func(h *LLRPHealth) { h.Connected = false; h.Disconnects++ })
			// A genuine established connection resets exponential backoff.
			backoff = retryMin
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		r.updateHealth(func(h *LLRPHealth) { h.LastError = err.Error() })
		t := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			if !t.Stop() {
				<-t.C
			}
			return ctx.Err()
		case <-t.C:
		}
		if backoff < retryMax/2 {
			backoff *= 2
		} else {
			backoff = retryMax
		}
	}
}

func (r *LLRPReader) readConnection(ctx context.Context, conn net.Conn, stream chan<- RFIDEvent, timeout, idleTimeout time.Duration) error {
	for {
		// Do not continue on a partial-frame timeout: byte alignment is lost.
		if err := conn.SetReadDeadline(time.Now().Add(idleTimeout)); err != nil {
			return err
		}
		header := make([]byte, llrpHeaderSize)
		if _, err := io.ReadFull(conn, header); err != nil {
			return fmt.Errorf("ERR_LLRP_READ: %w", err)
		}
		version := (header[0] >> 2) & 0x07
		typ := int(header[0]&0x03)<<8 | int(header[1])
		size := binary.BigEndian.Uint32(header[2:6])
		if header[0]&0xe0 != 0 || version != 1 || size < llrpHeaderSize || size > llrpMaxFrame {
			r.updateHealth(func(h *LLRPHealth) { h.RejectedFrames++ })
			return fmt.Errorf("ERR_LLRP_FRAME: version=%d length=%d", version, size)
		}
		payload := make([]byte, int(size)-llrpHeaderSize)
		if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return err
		}
		if _, err := io.ReadFull(conn, payload); err != nil {
			return fmt.Errorf("ERR_LLRP_PAYLOAD: %w", err)
		}
		switch typ {
		case llrpKeepalive:
			if size != llrpHeaderSize {
				r.updateHealth(func(h *LLRPHealth) { h.RejectedFrames++ })
				return errors.New("ERR_LLRP_KEEPALIVE_LENGTH")
			}
			ack := make([]byte, llrpHeaderSize)
			ack[0], ack[1] = 4, llrpKeepaliveAck
			binary.BigEndian.PutUint32(ack[2:6], llrpHeaderSize)
			copy(ack[6:10], header[6:10]) // echo reader's message ID
			if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
				return err
			}
			if n, err := conn.Write(ack); err != nil {
				return fmt.Errorf("ERR_LLRP_KEEPALIVE_ACK: %w", err)
			} else if n != len(ack) {
				return io.ErrShortWrite
			}
			r.updateHealth(func(h *LLRPHealth) { h.LastKeepaliveReceivedAt = time.Now().UTC() })
		case llrpROAccessReport:
			events, err := parseROAccessReport(payload)
			if err != nil {
				r.updateHealth(func(h *LLRPHealth) { h.RejectedFrames++ })
				return err
			}
			r.updateHealth(func(h *LLRPHealth) { h.LastReportReceivedAt = time.Now().UTC() })
			for _, event := range events {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case stream <- event:
					r.updateHealth(func(h *LLRPHealth) { h.ObservedTags++ })
				}
			}
		}
	}
}

type llrpParameter struct {
	typ     uint16
	payload []byte
}

// TV parameters are one-byte type markers plus fixed payload; TLVs carry their
// total four-byte-header-inclusive length. Unknown TV types cannot be safely skipped.
var tvSize = map[byte]int{1: 2, 2: 8, 3: 8, 4: 8, 5: 8, 6: 1, 7: 2, 8: 2, 9: 4, 10: 2, 13: 12, 14: 2, 16: 4}

func llrpParameters(buf []byte) ([]llrpParameter, error) {
	var params []llrpParameter
	for len(buf) != 0 {
		if buf[0]&0x80 != 0 {
			typ := buf[0] & 0x7f
			n, ok := tvSize[typ]
			if !ok || len(buf) < 1+n {
				return nil, fmt.Errorf("ERR_LLRP_TV: type=%d", typ)
			}
			params = append(params, llrpParameter{uint16(typ), buf[1 : 1+n]})
			buf = buf[1+n:]
			continue
		}
		if len(buf) < 4 {
			return nil, errors.New("ERR_LLRP_TLV_HEADER")
		}
		typ := binary.BigEndian.Uint16(buf[:2])
		n := int(binary.BigEndian.Uint16(buf[2:4]))
		if typ > 1023 || n < 4 || n > len(buf) {
			return nil, fmt.Errorf("ERR_LLRP_TLV_LENGTH: type=%d length=%d", typ, n)
		}
		params = append(params, llrpParameter{typ, buf[4:n]})
		buf = buf[n:]
	}
	return params, nil
}

func parseROAccessReport(payload []byte) ([]RFIDEvent, error) {
	outer, err := llrpParameters(payload)
	if err != nil {
		return nil, err
	}
	out := make([]RFIDEvent, 0)
	for _, p := range outer {
		if p.typ != llrpTagReport {
			continue
		}
		items, err := llrpParameters(p.payload)
		if err != nil {
			return nil, err
		}
		var e RFIDEvent
		seenEPC := false
		for _, item := range items {
			switch item.typ {
			case llrpEPC96:
				if seenEPC || len(item.payload) != 12 {
					return nil, errors.New("ERR_LLRP_DUPLICATE_EPC")
				}
				e.TagUID = "EPC-" + upperHex(item.payload)
				e.EPCBits, seenEPC = 96, true
			case llrpEPCData:
				if seenEPC || len(item.payload) < 3 {
					return nil, errors.New("ERR_LLRP_EPC_DATA")
				}
				bits := int(binary.BigEndian.Uint16(item.payload[:2]))
				n := (bits + 7) / 8
				if bits <= 0 || n != len(item.payload)-2 {
					return nil, errors.New("ERR_LLRP_EPC_BITS")
				}
				epc := append([]byte(nil), item.payload[2:]...)
				if bits%8 != 0 {
					mask := byte(0xff << (8 - bits%8))
					if epc[len(epc)-1]&^mask != 0 {
						return nil, errors.New("ERR_LLRP_EPC_PADDING")
					}
				}
				e.TagUID = "EPC-" + upperHex(epc)
				e.EPCBits, seenEPC = bits, true
			case 1: // AntennaID, TV
				e.AntennaID = int(binary.BigEndian.Uint16(item.payload))
				e.AntennaMeasured = true
			case 6: // PeakRSSI, TV, signed dBm
				e.RSSI = float64(int8(item.payload[0]))
				e.RSSIMeasured = true
			case 2: // FirstSeenTimestampUTC, TV, microseconds since UNIX epoch
				us := binary.BigEndian.Uint64(item.payload)
				if us > uint64(1<<63-1) {
					return nil, errors.New("ERR_LLRP_TIMESTAMP_RANGE")
				}
				observed := time.Unix(int64(us/1_000_000), int64(us%1_000_000)*1_000).UTC()
				e.Timestamp = &observed
				e.ReaderUTCMeasured = true
			}
		}
		if !seenEPC {
			return nil, errors.New("ERR_LLRP_TAG_WITHOUT_EPC")
		}
		out = append(out, e)
	}
	return out, nil
}
func upperHex(buf []byte) string {
	const chars = "0123456789ABCDEF"
	dst := make([]byte, hex.EncodedLen(len(buf)))
	for i, b := range buf {
		dst[2*i], dst[2*i+1] = chars[b>>4], chars[b&0xf]
	}
	return string(dst)
}
