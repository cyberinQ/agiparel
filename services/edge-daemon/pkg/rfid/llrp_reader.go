// Author: Sunil Jain, CyberinQ LLC
package rfid

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

type RFIDEvent struct {
	TagUID    string    `json:"tag_uid"`
	AntennaID int       `json:"antenna_id"`
	RSSI      float64   `json:"rssi"`
	Timestamp time.Time `json:"timestamp"`
}

type LLRPReader struct {
	Address string
	Timeout time.Duration
}

func NewLLRPReader(address string) *LLRPReader {
	return &LLRPReader{
		Address: address,
		Timeout: 5 * time.Second,
	}
}

func (r *LLRPReader) Listen(ctx context.Context, eventStream chan<- RFIDEvent) error {
	conn, err := net.DialTimeout("tcp", r.Address, r.Timeout)
	if err != nil {
		return fmt.Errorf("ERR_LLRP_DIAL: failed to connect to reader %s: %w", r.Address, err)
	}
	defer conn.Close()

	header := make([]byte, 10)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			_, err := io.ReadFull(conn, header)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return fmt.Errorf("ERR_LLRP_READ: connection broken: %w", err)
			}

			msgType := ((uint16(header[0]) & 0x03) << 8) | uint16(header[1])
			msgLen := binary.BigEndian.Uint32(header[2:6])
			if msgLen < 10 || msgLen > 1048576 {
				return fmt.Errorf("ERR_LLRP_FRAME: invalid message length %d", msgLen)
			}

			payload := make([]byte, msgLen-10)
			if _, err := io.ReadFull(conn, payload); err != nil {
				return fmt.Errorf("ERR_LLRP_PAYLOAD: failed to read full frame: %w", err)
			}

			if msgType == 61 {
				epcLength := min(len(payload), 12)
				eventStream <- RFIDEvent{
					TagUID:    fmt.Sprintf("EPC-%X", payload[:epcLength]),
					AntennaID: 1,
					RSSI:      -42.5,
					Timestamp: time.Now().UTC(),
				}
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
