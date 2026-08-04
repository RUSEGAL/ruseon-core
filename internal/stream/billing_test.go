package stream

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/RUSEGAL/REA-Stream-Engine/internal/config"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/storage"
)

func TestProcessBilling(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := storage.NewStorage(filepath.Join(tempDir, "db"))
	defer store.Close()

	// Initial cam configuration: used 100 bytes, last reset month is last month
	lastMonth := time.Now().AddDate(0, -1, 0).Format("2006-01")
	cam := &config.CameraConfig{
		ID:             "cam1",
		TrafficUsed:    100,
		LastResetMonth: lastMonth,
		TrafficLimit:   0,
	}
	_ = store.SaveCamera(cam)

	manager := NewManager()

	// Create a real Stream but don't start it fully (don't call NewStream, just construct)
	st := &Stream{
		ID: "cam1",
	}
	st.bytesReceived.Store(50)
	manager.streams["cam1"] = st

	lastBytes := make(map[string]uint64)
	lastBytes["cam1"] = 10

	// 1. First tick: should reset month, zero traffic, set default limit, add delta (50-10 = 40)
	processBilling(manager, store, lastBytes)

	c, _ := store.GetCamera("cam1")
	if c.TrafficLimit != 200*1024*1024*1024 {
		t.Errorf("expected default traffic limit to be set")
	}
	if c.LastResetMonth == lastMonth {
		t.Errorf("expected reset month to be updated")
	}
	if c.TrafficUsed != 40 {
		t.Errorf("expected traffic used to be 40 (reset to 0, then added 40), got %d", c.TrafficUsed)
	}
	if lastBytes["cam1"] != 50 {
		t.Errorf("expected lastBytes to be updated to 50")
	}

	// 2. Second tick: bytesReceived is smaller (stream restarted). Let's say bytesReceived is 15.
	// Delta logic says: if current < prev, traffic used += current (15)
	st.bytesReceived.Store(15)
	processBilling(manager, store, lastBytes)

	c, _ = store.GetCamera("cam1")
	if c.TrafficUsed != 55 { // 40 + 15
		t.Errorf("expected traffic used to be 55 (restarted stream), got %d", c.TrafficUsed)
	}
	if lastBytes["cam1"] != 15 {
		t.Errorf("expected lastBytes to be updated to 15")
	}

	// 3. Third tick: bytesReceived grew normally to 25. Delta is 10.
	st.bytesReceived.Store(25)
	processBilling(manager, store, lastBytes)

	c, _ = store.GetCamera("cam1")
	if c.TrafficUsed != 65 { // 55 + 10
		t.Errorf("expected traffic used to be 65, got %d", c.TrafficUsed)
	}
}
