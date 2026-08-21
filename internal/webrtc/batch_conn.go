package webrtc

import (
	"net"
	"net/netip"
	"time"
)

// BatchingUDPMuxConn wraps a *net.UDPConn and implements both net.PacketConn
// and AddrPortReaderWriter for zero-allocation, adaptive sendmmsg batching.
type BatchingUDPMuxConn struct {
	udpConn *net.UDPConn
	batcher *platformBatcher
}

// NewBatchingUDPMuxConn creates a new BatchingUDPMuxConn.
func NewBatchingUDPMuxConn(udpConn *net.UDPConn) *BatchingUDPMuxConn {
	return &BatchingUDPMuxConn{
		udpConn: udpConn,
		batcher: newPlatformBatcher(udpConn),
	}
}

// ReadFrom reads a packet from the connection.
func (c *BatchingUDPMuxConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	return c.udpConn.ReadFrom(p)
}

// WriteTo writes a packet with destination net.Addr.
func (c *BatchingUDPMuxConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		return c.udpConn.WriteTo(p, addr)
	}
	return c.WriteToAddrPort(p, udpAddr.AddrPort())
}

// ReadFromAddrPort implements ice.AddrPortReaderWriter for zero-alloc reads.
func (c *BatchingUDPMuxConn) ReadFromAddrPort(b []byte) (int, netip.AddrPort, error) {
	return c.udpConn.ReadFromUDPAddrPort(b)
}

// WriteToAddrPort implements ice.AddrPortReaderWriter for zero-alloc adaptive batching.
func (c *BatchingUDPMuxConn) WriteToAddrPort(b []byte, addr netip.AddrPort) (int, error) {
	return c.batcher.WriteToAddrPort(b, addr)
}

// LocalAddr returns the local network address.
func (c *BatchingUDPMuxConn) LocalAddr() net.Addr {
	return c.udpConn.LocalAddr()
}

// SetDeadline sets the read and write deadlines.
func (c *BatchingUDPMuxConn) SetDeadline(t time.Time) error {
	return c.udpConn.SetDeadline(t)
}

// SetReadDeadline sets the deadline for future Read calls.
func (c *BatchingUDPMuxConn) SetReadDeadline(t time.Time) error {
	return c.udpConn.SetReadDeadline(t)
}

// SetWriteDeadline sets the deadline for future Write calls.
func (c *BatchingUDPMuxConn) SetWriteDeadline(t time.Time) error {
	return c.udpConn.SetWriteDeadline(t)
}

// Close closes the batcher and underlying UDP connection.
func (c *BatchingUDPMuxConn) Close() error {
	if c.batcher != nil {
		c.batcher.Close()
	}
	return c.udpConn.Close()
}
