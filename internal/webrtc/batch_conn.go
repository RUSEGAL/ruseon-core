package webrtc

import (
	"net"
	"net/netip"
	"time"
)

// BatchingUDPMuxConn wraps a *net.UDPConn and implements both net.PacketConn
// and Pion ICE AddrPortReaderWriter for zero-allocation, adaptive sendmmsg packet batching.
type BatchingUDPMuxConn struct {
	udpConn *net.UDPConn
	batcher *platformBatcher
}

// NewBatchingUDPMuxConn wraps the provided UDPConn with platform-specific packet batching.
func NewBatchingUDPMuxConn(udpConn *net.UDPConn) *BatchingUDPMuxConn {
	return &BatchingUDPMuxConn{
		udpConn: udpConn,
		batcher: newPlatformBatcher(udpConn),
	}
}

// ReadFrom reads an incoming packet from the underlying UDP connection.
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

// ReadFromAddrPort implements ice.AddrPortReaderWriter for zero-allocation packet reading.
func (c *BatchingUDPMuxConn) ReadFromAddrPort(b []byte) (int, netip.AddrPort, error) {
	return c.udpConn.ReadFromUDPAddrPort(b)
}

// WriteToAddrPort implements ice.AddrPortReaderWriter for zero-allocation adaptive packet batching.
func (c *BatchingUDPMuxConn) WriteToAddrPort(b []byte, addr netip.AddrPort) (int, error) {
	return c.batcher.WriteToAddrPort(b, addr)
}

// LocalAddr returns the local network address of the UDP listener.
func (c *BatchingUDPMuxConn) LocalAddr() net.Addr {
	return c.udpConn.LocalAddr()
}

// SetDeadline sets the read and write deadlines on the UDP socket.
func (c *BatchingUDPMuxConn) SetDeadline(t time.Time) error {
	return c.udpConn.SetDeadline(t)
}

// SetReadDeadline sets the read deadline on the UDP socket.
func (c *BatchingUDPMuxConn) SetReadDeadline(t time.Time) error {
	return c.udpConn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline on the UDP socket.
func (c *BatchingUDPMuxConn) SetWriteDeadline(t time.Time) error {
	return c.udpConn.SetWriteDeadline(t)
}

// Close flushes all pending batches and closes the underlying UDP connection.
func (c *BatchingUDPMuxConn) Close() error {
	if c.batcher != nil {
		c.batcher.Close()
	}
	return c.udpConn.Close()
}
