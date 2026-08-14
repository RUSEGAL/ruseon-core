package webrtc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/RUSEGAL/ruseon-core/internal/buffer"
	"github.com/RUSEGAL/ruseon-core/internal/stream"
	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/RUSEGAL/ruseon-core/pkg/grpc/pb"
	"github.com/pion/webrtc/v4"
)

func TestWHEPHandler_MetadataDataChannel(t *testing.T) {
	// 1. Setup RingBuffer (needs SPS/PPS for WebRTC to start)
	rb := buffer.NewRingBuffer(100)
	rb.SetParams(nil, []byte{0x00, 0x00, 0x00, 0x01, 0x67}, []byte{0x00, 0x00, 0x00, 0x01, 0x68})

	// 2. Setup MetadataBroadcaster
	mb := stream.NewMetadataBroadcaster()

	// 3. Create Handler and Engine
	cfg := &config.Config{}
	cfg.Server.WebRTC.ICEServers = []string{"stun:stun.l.google.com:19302"}

	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	handler := NewWHEPHandler("test_stream", rb, mb, cfg, engine)

	// 4. Create Client WebRTC PeerConnection
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		t.Fatal(err)
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))
	clientPC, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	defer clientPC.Close()

	// Client creates transceiver for video so it expects video back
	_, err = clientPC.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 5. Setup Client DataChannel listener (and create it so it's in the SDP offer)
	dc, err := clientPC.CreateDataChannel("metadata", nil)
	if err != nil {
		t.Fatal(err)
	}
	
	msgChan := make(chan string, 1)
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		msgChan <- string(msg.Data)
	})

	// 6. Create Offer
	offer, err := clientPC.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Pion requires SetLocalDescription before ICE gathering starts
	gatherComplete := webrtc.GatheringCompletePromise(clientPC)
	if err := clientPC.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}
	<-gatherComplete

	// 7. Get Answer from Handler
	answerSDP, err := handler.HandleOffer(context.Background(), clientPC.LocalDescription().SDP)
	if err != nil {
		t.Fatalf("HandleOffer failed: %v", err)
	}

	// 8. Set Remote Description on Client
	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answerSDP,
	}
	if err := clientPC.SetRemoteDescription(answer); err != nil {
		t.Fatal(err)
	}

	// 9. Wait a bit for connection and DataChannel to open
	time.Sleep(500 * time.Millisecond)

	// 10. Push metadata
	req := &pb.MetadataRequest{
		CameraId: "test_stream",
		Pts:      42,
		Objects: []*pb.BoundingBox{
			{Label: "car", Confidence: 0.9},
		},
	}
	mb.Broadcast(req)

	// 11. Check if received
	select {
	case msg := <-msgChan:
		var receivedReq pb.MetadataRequest
		if err := json.Unmarshal([]byte(msg), &receivedReq); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}
		if receivedReq.Pts != 42 {
			t.Errorf("expected PTS 42, got %d", receivedReq.Pts)
		}
		if len(receivedReq.Objects) != 1 || receivedReq.Objects[0].Label != "car" {
			t.Errorf("unexpected objects: %v", receivedReq.Objects)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for metadata message on DataChannel")
	}
}
