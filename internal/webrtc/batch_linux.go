//go:build linux

package webrtc

import (
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	maxBatchSize  = 32
	maxPacketSize = 1500
	numShards     = 64
)

// peerBatch holds the pre-allocated batch buffers for a single peer destination.
type peerBatch struct {
	mu       sync.Mutex
	msgs     [maxBatchSize]ipv4.Message
	storage  [maxBatchSize][maxPacketSize]byte
	buffers  [maxBatchSize][]byte
	count    int
	udpAddr  *net.UDPAddr
	lastUsed int64 // unix timestamp in nanos
}

func newPeerBatch(addr netip.AddrPort) *peerBatch {
	pb := &peerBatch{
		udpAddr:  net.UDPAddrFromAddrPort(addr),
		lastUsed: time.Now().UnixNano(),
	}
	for i := range pb.msgs {
		pb.buffers[i] = pb.storage[i][:]
		pb.msgs[i].Buffers = [][]byte{pb.buffers[i]}
		pb.msgs[i].Addr = pb.udpAddr
	}
	return pb
}

type peerShard struct {
	mu    sync.RWMutex
	peers map[netip.AddrPort]*peerBatch
}

type platformBatcher struct {
	udpConn   *net.UDPConn
	pc4       *ipv4.PacketConn
	pc6       *ipv6.PacketConn
	shards    [numShards]peerShard
	closed    atomic.Bool
	stopClean chan struct{}
}

func newPlatformBatcher(udpConn *net.UDPConn) *platformBatcher {
	b := &platformBatcher{
		udpConn:   udpConn,
		stopClean: make(chan struct{}),
	}
	for i := range b.shards {
		b.shards[i].peers = make(map[netip.AddrPort]*peerBatch)
	}

	b.pc4 = ipv4.NewPacketConn(udpConn)
	b.pc6 = ipv6.NewPacketConn(udpConn)

	go b.cleanupLoop()

	return b
}

func (b *platformBatcher) getShard(addr netip.AddrPort) *peerShard {
	// Fast hash of AddrPort for sharding
	ip16 := addr.Addr().As16()
	hash := uint32(addr.Port())
	hash ^= uint32(ip16[12]) | (uint32(ip16[13]) << 8) | (uint32(ip16[14]) << 16) | (uint32(ip16[15]) << 24)
	return &b.shards[hash%numShards]
}

func (b *platformBatcher) getOrCreatePeerBatch(addr netip.AddrPort) *peerBatch {
	shard := b.getShard(addr)

	shard.mu.RLock()
	pb, exists := shard.peers[addr]
	shard.mu.RUnlock()

	if exists {
		return pb
	}

	shard.mu.Lock()
	pb, exists = shard.peers[addr]
	if !exists {
		pb = newPeerBatch(addr)
		shard.peers[addr] = pb
	}
	shard.mu.Unlock()

	return pb
}

func (b *platformBatcher) WriteToAddrPort(buf []byte, addr netip.AddrPort) (int, error) {
	if b.closed.Load() {
		return 0, net.ErrClosed
	}

	if len(buf) == 0 {
		return 0, nil
	}

	// 1. Non-RTP packet (STUN, DTLS handshake, etc. - length < 12 or version != 2)
	// Send immediately without batching to prevent handshake latency
	if len(buf) < 12 || (buf[0]&0xC0) != 0x80 {
		pb := b.getOrCreatePeerBatch(addr)
		pb.mu.Lock()
		if pb.count > 0 {
			_ = b.flushLocked(pb, addr.Addr().Is6())
		}
		pb.mu.Unlock()
		return b.udpConn.WriteToUDPAddrPort(buf, addr)
	}

	// 2. RTP packet: Check Marker bit (RFC 6184: M=1 is the final packet of a video frame)
	isMarker := (buf[1] & 0x80) != 0

	pb := b.getOrCreatePeerBatch(addr)
	pb.mu.Lock()
	defer pb.mu.Unlock()

	atomic.StoreInt64(&pb.lastUsed, time.Now().UnixNano())

	// If packet is larger than MTU buffer, flush batch first and send directly
	if len(buf) > maxPacketSize {
		if pb.count > 0 {
			_ = b.flushLocked(pb, addr.Addr().Is6())
		}
		return b.udpConn.WriteToUDPAddrPort(buf, addr)
	}

	// Copy into pre-allocated slot
	n := copy(pb.storage[pb.count][:], buf)
	pb.msgs[pb.count].Buffers[0] = pb.storage[pb.count][:n]
	pb.count++

	// 3. Trigger Flush if:
	// a) Marker bit is set (End of video frame Access Unit)
	// b) Batch capacity is full (32 packets)
	if isMarker || pb.count >= maxBatchSize {
		if err := b.flushLocked(pb, addr.Addr().Is6()); err != nil {
			return n, err
		}
	}

	return n, nil
}

func (b *platformBatcher) flushLocked(pb *peerBatch, isIPv6 bool) error {
	if pb.count == 0 {
		return nil
	}

	total := pb.count
	sent := 0

	for sent < total {
		var n int
		var err error

		switch {
		case isIPv6 && b.pc6 != nil:
			n, err = b.pc6.WriteBatch(pb.msgs[sent:total], 0)
		case b.pc4 != nil:
			n, err = b.pc4.WriteBatch(pb.msgs[sent:total], 0)
		default:
			// Fallback if raw packet conn unavailable
			for i := sent; i < total; i++ {
				_, writeErr := b.udpConn.WriteToUDPAddrPort(pb.msgs[i].Buffers[0], pb.udpAddr.AddrPort())
				if writeErr != nil {
					pb.count = 0
					return writeErr
				}
			}
			pb.count = 0
			return nil
		}

		if err != nil {
			pb.count = 0
			return err
		}

		if n <= 0 {
			break
		}
		sent += n
	}

	pb.count = 0
	return nil
}

func (b *platformBatcher) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopClean:
			return
		case <-ticker.C:
			now := time.Now().UnixNano()
			// Prune inactive peers (no traffic for > 30s)
			for i := range b.shards {
				shard := &b.shards[i]
				shard.mu.Lock()
				for addr, pb := range shard.peers {
					pb.mu.Lock()
					if pb.count > 0 {
						_ = b.flushLocked(pb, addr.Addr().Is6())
					}
					last := atomic.LoadInt64(&pb.lastUsed)
					if now-last > int64(30*time.Second) {
						delete(shard.peers, addr)
					}
					pb.mu.Unlock()
				}
				shard.mu.Unlock()
			}
		}
	}
}

func (b *platformBatcher) Close() {
	if b.closed.CompareAndSwap(false, true) {
		close(b.stopClean)
		// Flush all remaining batches
		for i := range b.shards {
			shard := &b.shards[i]
			shard.mu.Lock()
			for addr, pb := range shard.peers {
				pb.mu.Lock()
				if pb.count > 0 {
					_ = b.flushLocked(pb, addr.Addr().Is6())
				}
				pb.mu.Unlock()
			}
			shard.peers = make(map[netip.AddrPort]*peerBatch)
			shard.mu.Unlock()
		}
	}
}
