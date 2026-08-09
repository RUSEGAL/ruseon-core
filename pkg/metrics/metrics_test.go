package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsRegistration(t *testing.T) {
	// 1. Test Gauges
	ActiveStreams.Set(10)
	if val := testutil.ToFloat64(ActiveStreams); val != 10 {
		t.Errorf("Expected ActiveStreams to be 10, got %v", val)
	}
	ActiveStreams.Set(0) // Reset

	WebRTCPeersActive.Set(5)
	if val := testutil.ToFloat64(WebRTCPeersActive); val != 5 {
		t.Errorf("Expected WebRTCPeersActive to be 5, got %v", val)
	}
	WebRTCPeersActive.Set(0) // Reset

	// 2. Test Counters
	FramesReceivedTotal.Inc()
	if val := testutil.ToFloat64(FramesReceivedTotal); val != 1 {
		t.Errorf("Expected FramesReceivedTotal to be 1, got %v", val)
	}

	NetworkReceiveBytesTotal.Add(1024)
	if val := testutil.ToFloat64(NetworkReceiveBytesTotal); val != 1024 {
		t.Errorf("Expected NetworkReceiveBytesTotal to be 1024, got %v", val)
	}

	EventbusDropsTotal.Inc()
	if val := testutil.ToFloat64(EventbusDropsTotal); val != 1 {
		t.Errorf("Expected EventbusDropsTotal to be 1, got %v", val)
	}

	RingbufferDropsTotal.Inc()
	if val := testutil.ToFloat64(RingbufferDropsTotal); val != 1 {
		t.Errorf("Expected RingbufferDropsTotal to be 1, got %v", val)
	}

	HLSRequestsTotal.Inc()
	if val := testutil.ToFloat64(HLSRequestsTotal); val != 1 {
		t.Errorf("Expected HLSRequestsTotal to be 1, got %v", val)
	}

	// 3. Test Registry via Gatherer (ensure metrics are exported correctly)
	gatherer := prometheus.DefaultGatherer
	mfs, err := gatherer.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	foundActiveStreams := false
	for _, mf := range mfs {
		if strings.HasPrefix(*mf.Name, "ruseon_") {
			if *mf.Name == "ruseon_active_streams" {
				foundActiveStreams = true
			}
		}
	}

	if !foundActiveStreams {
		t.Errorf("Expected ruseon_active_streams to be registered in DefaultGatherer")
	}
}
