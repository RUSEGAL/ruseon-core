package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockStateStore struct{ StateStore }
type mockAuthenticator struct{ Authenticator }
type mockBlobStore struct{ BlobStore }
type mockEventBus struct{ EventBus }

func TestRegistry_Registrations(t *testing.T) {
	origState := CurrentStateStore
	origAuth := CurrentAuthenticator
	origBlob := CurrentBlobStore
	origBus := CurrentEventBus
	defer func() {
		CurrentStateStore = origState
		CurrentAuthenticator = origAuth
		CurrentBlobStore = origBlob
		CurrentEventBus = origBus
	}()

	ms := &mockStateStore{}
	RegisterStateStore(ms)
	assert.Equal(t, ms, CurrentStateStore)

	ma := &mockAuthenticator{}
	RegisterAuthenticator(ma)
	assert.Equal(t, ma, CurrentAuthenticator)

	mb := &mockBlobStore{}
	RegisterBlobStore(mb)
	assert.Equal(t, mb, CurrentBlobStore)

	me := &mockEventBus{}
	RegisterEventBus(me)
	assert.Equal(t, me, CurrentEventBus)
}
