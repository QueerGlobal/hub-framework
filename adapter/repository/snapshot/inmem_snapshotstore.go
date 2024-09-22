package snapshot

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"sync"
	"time"

	"github.com/hashicorp/raft"
)

// InmemSnapshotStore implements the SnapshotStore interface and
// retains only the most recent snapshot
type InmemSnapshotStore struct {
	latest      *InmemSnapshotSink
	hasSnapshot bool
	sync.RWMutex
}

// InmemSnapshotSink implements SnapshotSink in memory
type InmemSnapshotSink struct {
	meta     raft.SnapshotMeta
	contents *bytes.Buffer
}

// NewInmemSnapshotStore creates a blank new InmemSnapshotStore
func NewInmemSnapshotStore() *InmemSnapshotStore {
	return &InmemSnapshotStore{
		latest: &InmemSnapshotSink{
			contents: &bytes.Buffer{},
		},
	}
}

// Create replaces the stored snapshot with a new one using the given args
func (m *InmemSnapshotStore) Create(version raft.SnapshotVersion, index,
	term uint64, configuration raft.Configuration,
	configurationIndex uint64, trans raft.Transport) (raft.SnapshotSink, error) {
	// We only support version 1 snapshots at this time.
	if version != 1 {
		return nil, fmt.Errorf("unsupported snapshot version %d", version)
	}

	name := snapshotName(term, index)

	m.Lock()
	defer m.Unlock()

	sink := &InmemSnapshotSink{
		meta: raft.SnapshotMeta{
			Version: version,
			ID:      name,
			Index:   index,
			Term:    term,
			//TODO: Get peers... Peers:              //encodePeers(configuration, trans),
			Configuration:      configuration,
			ConfigurationIndex: configurationIndex,
		},
		contents: &bytes.Buffer{},
	}
	m.hasSnapshot = true
	m.latest = sink

	return sink, nil
}

// List returns the latest snapshot taken
func (m *InmemSnapshotStore) List() ([]*raft.SnapshotMeta, error) {
	m.RLock()
	defer m.RUnlock()

	if !m.hasSnapshot {
		return []*raft.SnapshotMeta{}, nil
	}
	return []*raft.SnapshotMeta{&m.latest.meta}, nil
}

// Open wraps an io.ReadCloser around the snapshot contents
func (m *InmemSnapshotStore) Open(id string) (*raft.SnapshotMeta, io.ReadCloser, error) {
	m.RLock()
	defer m.RUnlock()

	if m.latest.meta.ID != id {
		return nil, nil, fmt.Errorf("[ERR] snapshot: failed to open snapshot id: %s", id)
	}

	// Make a copy of the contents, since a bytes.Buffer can only be read
	// once.
	contents := bytes.NewBuffer(m.latest.contents.Bytes())
	return &m.latest.meta, ioutil.NopCloser(contents), nil
}

// Write appends the given bytes to the snapshot contents
func (s *InmemSnapshotSink) Write(p []byte) (n int, err error) {
	written, err := s.contents.Write(p)
	s.meta.Size += int64(written)
	return written, err
}

// Close updates the Size and is otherwise a no-op
func (s *InmemSnapshotSink) Close() error {
	return nil
}

// ID returns the ID of the SnapshotMeta
func (s *InmemSnapshotSink) ID() string {
	return s.meta.ID
}

// Cancel returns successfully with a nil error
func (s *InmemSnapshotSink) Cancel() error {
	return nil
}

// snapshotName generates a name for the snapshot.
func snapshotName(term, index uint64) string {
	now := time.Now()
	msec := now.UnixNano() / int64(time.Millisecond)
	return fmt.Sprintf("%d-%d-%d", term, index, msec)
}
