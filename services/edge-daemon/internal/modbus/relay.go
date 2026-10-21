package modbus

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"
)

type RelayController struct {
	mu      sync.Mutex
	addr    string
	timeout time.Duration
}

func NewRelayController(addr string) *RelayController {
	return &RelayController{
		addr:    addr,
		timeout: 50 * time.Millisecond,
	}
}

// TripMotorInterlock writes a Modbus TCP discrete coil command to cut sewing motor power (< 15ms)
func (r *RelayController) TripMotorInterlock(coilIndex uint16) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	conn, err := net.DialTimeout("tcp", r.addr, r.timeout)
	if err != nil {
		return fmt.Errorf("modbus relay unreachable at %s: %w", r.addr, err)
	}
	defer conn.Close()

	// Modbus TCP ADU: Transaction ID (2B), Protocol ID (2B), Length (2B), Unit ID (1B), Function Code (1B), Coil Addr (2B), Output Value (2B)
	req := make([]byte, 12)
	binary.BigEndian.PutUint16(req[0:2], 0x0001)  // Transaction ID
	binary.BigEndian.PutUint16(req[2:4], 0x0000)  // Modbus Protocol
	binary.BigEndian.PutUint16(req[4:6], 0x0006)  // Length
	req[6] = 0x01                                 // Unit ID
	req[7] = 0x05                                 // Function Code: Write Single Coil
	binary.BigEndian.PutUint16(req[8:10], coilIndex) // Coil Address
	binary.BigEndian.PutUint16(req[10:12], 0xFF00)   // 0xFF00 = Force Coil ON (Trip Relay)

	_ = conn.SetDeadline(time.Now().Add(r.timeout))
	_, err = conn.Write(req)
	return err
}