package stream

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RUSEGAL/ruseon-core/v2/pkg/config"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/storage"
)

func TestBilling_DeltaAndBatchFlush(t *testing.T) {
	tempDir := t.TempDir()
	store, err := storage.NewStorage(filepath.Join(tempDir, "db"))
	require.NoError(t, err)
	defer store.Close()

	// Initial cam configuration: used 100 bytes, last reset month is last month
	lastMonth := time.Now().AddDate(0, -1, 0).Format("2006-01")
	cam := &config.CameraConfig{
		ID:             "cam1",
		TrafficUsed:    100,
		LastResetMonth: lastMonth,
		TrafficLimit:   0,
	}
	require.NoError(t, store.SaveCamera(cam))

	manager := NewManager()
	defer manager.Close()

	st := &Stream{ID: "cam1"}
	st.bytesReceived.Store(50)
	manager.streams["cam1"] = st

	lastBytes := make(map[string]uint64)
	lastBytes["cam1"] = 10
	pendingDelta := make(map[string]uint64)

	// 1. Collect delta: (50 - 10 = 40)
	collectBillingDelta(manager, lastBytes, pendingDelta)
	assert.Equal(t, uint64(40), pendingDelta["cam1"])
	assert.Equal(t, uint64(50), lastBytes["cam1"])

	// 2. Flush to DB
	err = flushBilling(store, pendingDelta)
	require.NoError(t, err)

	c, err := store.GetCamera("cam1")
	require.NoError(t, err)
	assert.Equal(t, uint64(200*1024*1024*1024), c.TrafficLimit)
	assert.Equal(t, time.Now().Format("2006-01"), c.LastResetMonth)
	assert.Equal(t, uint64(40), c.TrafficUsed)

	// Clear pendingDelta
	delete(pendingDelta, "cam1")

	// 3. Second collect: stream restarted, bytesReceived is 15
	st.bytesReceived.Store(15)
	collectBillingDelta(manager, lastBytes, pendingDelta)
	assert.Equal(t, uint64(15), pendingDelta["cam1"])
	assert.Equal(t, uint64(15), lastBytes["cam1"])

	// 4. Flush second delta
	err = flushBilling(store, pendingDelta)
	require.NoError(t, err)

	c, err = store.GetCamera("cam1")
	require.NoError(t, err)
	assert.Equal(t, uint64(55), c.TrafficUsed) // 40 + 15
}

func TestStartBillingTask(t *testing.T) {
	tempDir := t.TempDir()
	store, err := storage.NewStorage(filepath.Join(tempDir, "db"))
	require.NoError(t, err)
	defer store.Close()

	manager := NewManager()
	defer manager.Close()

	_ = store.SaveCamera(&config.CameraConfig{ID: "cam_task", TrafficUsed: 0})
	st := &Stream{ID: "cam_task"}
	st.bytesReceived.Store(100)
	manager.streams["cam_task"] = st

	ctx, cancel := context.WithCancel(context.Background())
	StartBillingTask(ctx, nil, manager, store)

	// Cancel to trigger shutdown flush
	time.Sleep(20 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)

	c, err := store.GetCamera("cam_task")
	require.NoError(t, err)
	assert.Equal(t, uint64(100), c.TrafficUsed)
}
