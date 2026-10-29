// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package rfid

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"
)

func param(typ uint16, data ...byte) []byte {
	out := make([]byte, 4+len(data))
	binary.BigEndian.PutUint16(out[:2], typ)
	binary.BigEndian.PutUint16(out[2:4], uint16(len(out)))
	copy(out[4:], data)
	return out
}
func tv(typ byte, data ...byte) []byte { return append([]byte{0x80 | typ}, data...) }
func frame(typ byte, payload []byte, id uint32) []byte {
	out := make([]byte, 10+len(payload))
	out[0] = 4
	out[1] = typ
	binary.BigEndian.PutUint32(out[2:6], uint32(len(out)))
	binary.BigEndian.PutUint32(out[6:10], id)
	copy(out[10:], payload)
	return out
}
func tag(elems ...[]byte) []byte {
	var b []byte
	for _, e := range elems {
		b = append(b, e...)
	}
	return param(240, b...)
}
func TestLLRPReportRealEPCAndOptionalFields(t *testing.T) {
	epc := []byte{0x30, 0x00, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0xab, 0xcd}
	ts := make([]byte, 8)
	binary.BigEndian.PutUint64(ts, 1710000000123456)
	payload := append(tag(tv(13, epc...), tv(1, 0, 7), tv(6, 0xe2), tv(2, ts...)), tag(tv(13, epc...))...)
	events, err := parseROAccessReport(payload)
	if err != nil || len(events) != 2 {
		t.Fatalf("parse reports: %v %v", events, err)
	}
	e := events[0]
	if e.TagUID != "EPC-30001234567890ABCDEFABCD" || e.EPCBits != 96 || !e.AntennaMeasured || e.AntennaID != 7 || !e.RSSIMeasured || e.RSSI != -30 || !e.ReaderUTCMeasured || e.Timestamp.UnixMicro() != 1710000000123456 {
		t.Fatalf("measured fields wrong: %+v", e)
	}
	blank := events[1]
	if blank.AntennaID != 0 || blank.RSSI != 0 || blank.AntennaMeasured || blank.RSSIMeasured || blank.ReaderUTCMeasured || blank.Timestamp != nil {
		t.Fatalf("invented optional measurements: %+v", blank)
	}
}
func TestLLRPVariableEPCAndMalformedNoInventedTag(t *testing.T) {
	data := append([]byte{0, 40}, []byte{0x11, 0x22, 0x33, 0x44, 0x55}...)
	events, err := parseROAccessReport(tag(param(241, data...)))
	if err != nil || len(events) != 1 || events[0].EPCBits != 40 || events[0].TagUID != "EPC-1122334455" {
		t.Fatalf("EPCData %v %v", events, err)
	}
	// Padding bits are forbidden in the last byte; otherwise distinct tags collapse.
	for _, input := range [][]byte{
		tag(tv(1, 0, 1)),
		tag(tv(13, 0, 1)),
		tag(append(tv(13, make([]byte, 12)...), tv(13, make([]byte, 12)...)...)),
		tag(param(241, 0, 9, 0x12, 0xff)),
		tag([]byte{0x8f}),     // unknown TV cannot be skipped safely
		{0, 240, 0, 3},        // impossible TLV length
		{0, 240, 0, 20, 0x8d}, // truncated TLV
	} {
		if got, err := parseROAccessReport(input); err == nil {
			t.Fatalf("malformed report emitted %v", got)
		}
	}
	if got, err := parseROAccessReport(nil); err != nil || len(got) != 0 {
		t.Fatalf("empty frame: %v %v", got, err)
	}
}
func TestLLRPTCPKeepaliveAckAndReconnect(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	reader := NewLLRPReader(listener.Addr().String())
	reader.Timeout = 200 * time.Millisecond
	reader.RetryMin = 10 * time.Millisecond
	reader.RetryMax = 50 * time.Millisecond
	output := make(chan RFIDEvent, 4)
	done := make(chan error, 1)
	go func() { done <- reader.Listen(ctx, output) }()
	epc := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	for round := 0; round < 2; round++ {
		if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatal(err)
		}
		conn, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		if round == 0 {
			if _, err = conn.Write(frame(62, nil, 42)); err != nil {
				t.Fatal(err)
			}
			ack := make([]byte, 10)
			if err = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
				t.Fatal(err)
			}
			if _, err = readExact(conn, ack); err != nil {
				t.Fatal(err)
			}
			if ack[0] != 4 || ack[1] != 72 || binary.BigEndian.Uint32(ack[2:6]) != 10 || binary.BigEndian.Uint32(ack[6:10]) != 42 {
				t.Fatalf("bad keepalive ack %v", ack)
			}
		}
		if _, err = conn.Write(frame(61, tag(tv(13, epc...)), uint32(round+10))); err != nil {
			t.Fatal(err)
		}
		select {
		case e := <-output:
			if e.TagUID != "EPC-0102030405060708090A0B0C" {
				t.Fatal(e)
			}
		case <-ctx.Done():
			t.Fatal("timed out waiting for tag")
		}
		conn.Close()
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, %v", err)
	}
	h := reader.Health()
	if h.Connects < 2 || h.ObservedTags != 2 || h.LastKeepaliveReceivedAt.IsZero() {
		t.Fatalf("health %+v", h)
	}
}
func readExact(c net.Conn, b []byte) (int, error) {
	n := 0
	for n < len(b) {
		m, e := c.Read(b[n:])
		n += m
		if e != nil {
			return n, e
		}
	}
	return n, nil
}
func TestLLRPListenCancelledBlockedSend(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r := NewLLRPReader(listener.Addr().String())
	r.RetryMin = 10 * time.Millisecond
	done := make(chan error, 1)
	go func() { done <- r.Listen(ctx, make(chan RFIDEvent)) }()
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, err = conn.Write(frame(61, tag(tv(13, make([]byte, 12)...)), 7))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if err = <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("blocked send did not cancel: %v", err)
	}
}
func TestLLRPMalformedFrameRejects(t *testing.T) {
	for _, input := range [][]byte{
		frame(61, []byte{0, 240, 0, 3}, 1),
		{4, 61, 0, 0, 0, 5, 0, 0, 0, 1},
		{0xff, 61, 0, 0, 0, 10, 0, 0, 0, 1},
	} {
		l, _ := net.Listen("tcp", "127.0.0.1:0")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		r := NewLLRPReader(l.Addr().String())
		r.Timeout = 100 * time.Millisecond
		r.RetryMin = 10 * time.Millisecond
		done := make(chan error, 1)
		go func() { done <- r.Listen(ctx, make(chan RFIDEvent, 1)) }()
		c, err := l.Accept()
		if err != nil {
			t.Fatal(err)
		}
		c.Write(input)
		c.Close()
		time.Sleep(30 * time.Millisecond)
		cancel()
		<-done
		l.Close()
		h := r.Health()
		if h.RejectedFrames < 1 {
			t.Fatalf("malformed frame not counted: %+v", h)
		}
	}
}
