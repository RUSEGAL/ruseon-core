package webrtc

import (
	"bytes"
	"net"
	"net/netip"
	"sync"
	"testing"
)

func TestBatchingUDPMuxConn_InterfaceImplementation(t *testing.T) {
	l, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("failed to listen UDP: %v", err)
	}
	defer l.Close()

	conn := NewBatchingUDPMuxConn(l)
	defer conn.Close()

	var _ net.PacketConn = conn
}

func TestBatchingUDPMuxConn_MarkerBitDetection(t *testing.T) {
	server, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("failed to listen UDP server: %v", err)
	}
	defer server.Close()

	serverAddrPort := netip.MustParseAddrPort(server.LocalAddr().String())

	client, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("failed to listen UDP client: %v", err)
	}
	defer client.Close()

	batchConn := NewBatchingUDPMuxConn(client)
	defer batchConn.Close()

	// Create 3 RTP packets for a P-frame: 2 with Marker=0, 1 with Marker=1
	pkt1 := make([]byte, 100)
	pkt1[0] = 0x80 // Version 2
	pkt1[1] = 0x60 // Marker=0, PT=96

	pkt2 := make([]byte, 100)
	pkt2[0] = 0x80
	pkt2[1] = 0x60 // Marker=0, PT=96

	pkt3 := make([]byte, 100)
	pkt3[0] = 0x80
	pkt3[1] = 0xE0 // Marker=1 (0x80 | 0x60), PT=96

	var wg sync.WaitGroup
	wg.Add(1)

	received := make([][]byte, 0, 3)
	var recMu sync.Mutex

	go func() {
		defer wg.Done()
		buf := make([]byte, 1500)
		for len(received) < 3 {
			n, _, readErr := server.ReadFromUDPAddrPort(buf)
			if readErr != nil {
				return
			}
			data := make([]byte, n)
			copy(data, buf[:n])
			recMu.Lock()
			received = append(received, data)
			recMu.Unlock()
		}
	}()

	// Send pkt1, pkt2, pkt3
	if _, err := batchConn.WriteToAddrPort(pkt1, serverAddrPort); err != nil {
		t.Fatalf("write pkt1 failed: %v", err)
	}
	if _, err := batchConn.WriteToAddrPort(pkt2, serverAddrPort); err != nil {
		t.Fatalf("write pkt2 failed: %v", err)
	}
	if _, err := batchConn.WriteToAddrPort(pkt3, serverAddrPort); err != nil {
		t.Fatalf("write pkt3 failed: %v", err)
	}

	wg.Wait()

	recMu.Lock()
	defer recMu.Unlock()
	if len(received) != 3 {
		t.Fatalf("expected 3 received packets, got %d", len(received))
	}
	if !bytes.Equal(received[0], pkt1) || !bytes.Equal(received[1], pkt2) || !bytes.Equal(received[2], pkt3) {
		t.Errorf("received packet contents mismatch")
	}
}

func TestBatchingUDPMuxConn_NonRTPImmediateSend(t *testing.T) {
	server, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("failed to listen UDP server: %v", err)
	}
	defer server.Close()

	serverAddrPort := netip.MustParseAddrPort(server.LocalAddr().String())

	client, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("failed to listen UDP client: %v", err)
	}
	defer client.Close()

	batchConn := NewBatchingUDPMuxConn(client)
	defer batchConn.Close()

	// STUN / Non-RTP packet (< 12 bytes or version != 2)
	stunPkt := []byte{0x00, 0x01, 0x00, 0x08}

	if _, err := batchConn.WriteToAddrPort(stunPkt, serverAddrPort); err != nil {
		t.Fatalf("write stun failed: %v", err)
	}

	buf := make([]byte, 100)
	n, _, err := server.ReadFromUDPAddrPort(buf)
	if err != nil {
		t.Fatalf("read stun failed: %v", err)
	}
	if !bytes.Equal(buf[:n], stunPkt) {
		t.Errorf("stun packet mismatch: got %v, expected %v", buf[:n], stunPkt)
	}
}

func BenchmarkBatchingUDPMuxConn_WriteToAddrPort(b *testing.B) {
	server, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		b.Fatalf("failed to listen: %v", err)
	}
	defer server.Close()

	// Drain goroutine
	go func() {
		buf := make([]byte, 1500)
		for {
			_, _, readErr := server.ReadFromUDPAddrPort(buf)
			if readErr != nil {
				return
			}
		}
	}()

	serverAddrPort := netip.MustParseAddrPort(server.LocalAddr().String())

	client, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		b.Fatalf("failed to listen client: %v", err)
	}
	defer client.Close()

	batchConn := NewBatchingUDPMuxConn(client)
	defer batchConn.Close()

	pkt := make([]byte, 1200)
	pkt[0] = 0x80
	pkt[1] = 0xE0 // Marker=1

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = batchConn.WriteToAddrPort(pkt, serverAddrPort)
	}
}
