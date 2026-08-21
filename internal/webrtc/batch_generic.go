//go:build !linux

package webrtc

import (
	"net"
	"net/netip"
)

type platformBatcher struct {
	udpConn *net.UDPConn
}

func newPlatformBatcher(udpConn *net.UDPConn) *platformBatcher {
	return &platformBatcher{
		udpConn: udpConn,
	}
}

func (b *platformBatcher) WriteToAddrPort(buf []byte, addr netip.AddrPort) (int, error) {
	return b.udpConn.WriteToUDPAddrPort(buf, addr)
}

func (b *platformBatcher) Close() {
}
