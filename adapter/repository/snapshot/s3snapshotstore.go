package snapshot

//TODO Implement and uncomment this.

/*
import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"context"
	"errors"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/session"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/plugin/pb"
	"github.com/rboyer/safeio"

	s3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/hashicorp/raft"
	"go.uber.org/atomic"
)


   type S3SnapshotSink struct {

   }

   func NewS3SnapshotSink(bucketName string, prefix string,
   	svc s3.Session, ID string) *S3SnapshotSink {

   	snapshot := S3SnapshotSink{
   		bucket: bucketName,
   		svc:    s3.New(session.New()),
   		cancel: make(chan bool),
   	}
   	return &snapshot
   }

   func (s *S3SnapshotSink) Write(p []byte) (n int, err error) {

   	success := false
   	unixtime := time.Now().UTC().Unix()
   	timestring := strconv.FormatInt(unixtime, 10)
   	filename := s.ID() + timestring + ".json"

   	for !success {

   		input := &s3.PutObjectInput{
   			Body:                 aws.ReadSeekCloser(strings.NewReader(filename)),
   			Bucket:               aws.String(s.bucket),
   			Key:                  aws.String(filename),
   			ServerSideEncryption: aws.String("AES256"),
   			StorageClass:         aws.String("STANDARD_IA"),
   		}

   		_, err = s.svc.PutObject(input)
   		if err != nil {
   			if aerr, ok := err.(awserr.Error); ok {
   				switch aerr.Code() {
   				default:
   					fmt.Println(aerr.Error())
   				}
   			} else {
   				// Print the error, cast err to
   				// awserr.Error to get the Code and
   				// Message from an error.
   				fmt.Println(err.Error())
   			}
   		} else {
   			success = true
   		}

   	}
   	return len(p), nil
   }

   func (s *S3SnapshotSink) Close() error {
   	return nil
   }

   func (s *S3SnapshotSink) ID() string {
   	return ID
   }

   func (s *S3SnapshotSink) Cancel() error {
   	return nil
   }






   type S3SnapshotStore struct {
   	bucket string
   	prefix string
   	svc    s3.Session
   }

   // Create is used to begin a snapshot at a given index and term, and with
   // the given committed configuration. The version parameter controls
   // which snapshot version to create.
   func (store *S3SnapshotStore) Create(version SnapshotVersion, index, term uint64, configuration Configuration,
   	configurationIndex uint64, trans Transport) (SnapshotSink, error) {

   		sink := NewS3SnapshotSink(store.bucketName, store.prefix,
   			store.svc, ID)

   			snapshot := S3SnapshotSink{
   				bucket: bucketName,

   				cancel: make(chan bool),
   			}
   			return &snapshot
   		}
   		//TODO: Create Snapshot Sink here... How should index play into sink ID?
   		// Look into configuration
   }

   func (store *S3SnapshotStore) generateSnapshotID(index uint64) string {

   	epoch := time.Now().Unix()
   	epochStr := strconv.Itoa(epoch)

   	id := uuid.New()
   	snapshotID := "snap-" + epochStr + "-" + id.String()
   	return snapshotID
   }


   // List is used to list the available snapshots in the store.
   // It should return then in descending order, with the highest index first.
   func (store *S3SnapshotStore) List() ([]*SnapshotMeta, error) {


   }

   	// Open takes a snapshot ID and provides a ReadCloser. Once close is
   	// called it is assumed the snapshot is no longer needed.
   func (store *S3SnapshotStore) Open(id string)
   	(*SnapshotMeta, io.ReadCloser, error) {

   	}



/*
const (
	// s3SnapshotID is the stable ID for any s3 snapshot. Keeping the ID
	// stable means there is only ever one s3 snapshot in the system
	s3SnapshotID = "s3-snapshot"
	tmpSuffix    = ".tmp"
	snapPath     = "snapshots"
)

// S3SnapshotStore implements the SnapshotStore interface and allows snapshots
// to be stored in files on S3. Since we always have an up to
// date FSM we use a special snapshot ID to indicate that the snapshot can be
// pulled from the S3
//
// When a snapshot is being installed on the node we will Create and Write data
// to it. This will cause the snapshot store to create a new S3DB file and
// write the snapshot data to it. Then, we can simply rename the snapshot to the
// FSM's filename. This allows us to atomically install the snapshot and
// reduces the amount of disk i/o. Older snapshots are reaped on startup and
// before each subsequent snapshot write. This ensures we only have one snapshot
// on disk at a time.
type S3SnapshotStore struct {
	// path is the directory in which to store file based snapshots
	path string

	// bucket is the s3 bucket to write to
	bucket string

	// prefix is the path within the s3 bucket to write to.
	prefix string

	// svc is our s3 client session object
	svc s3.Session

	fsm *raft.FSM

	config *raft.Config

	logger log.Logger
}

// S3SnapshotSink implements SnapshotSink optionally choosing to write to a
// file.
type S3SnapshotSink struct {
	store  *S3SnapshotStore
	logger log.Logger
	meta   raft.SnapshotMeta
	trans  raft.Transport

	// These fields will be used if we are writing a snapshot (vs. reading
	// one)
	written       atomic.Bool
	writer        io.WriteCloser
	writeError    error
	dir           string
	parentDir     string
	doneWritingCh chan struct{}

	l      sync.Mutex
	closed bool
}

// NewS3SnapshotStore creates a new S3SnapshotStore based
// on a base directory.
func NewS3SnapshotStore(base, bucket, prefix string, logger log.Logger, fsm *raft.FSM) (*S3SnapshotStore, error) {
	if logger == nil {
		return nil, fmt.Errorf("no logger provided")
	}

	// Ensure our path exists
	path := filepath.Join(base, snapPath)
	if err := os.MkdirAll(path, 0o755); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("snapshot path not accessible: %v", err)
	}
	// Setup the store
	store := &S3SnapshotStore{
		path:   path,
		bucket: bucket,
		prefix: prefix,
		svc:    s3.New(session.New()),
		fsm:    fsm,
		logger: logger,
	}

	// Cleanup any old or failed snapshots on startup.
	if err := store.ReapSnapshots(); err != nil {
		return nil, err
	}
	return store, nil
}

// Create is used to start a new snapshot
func (f *S3SnapshotStore) Create(version raft.SnapshotVersion, index, term uint64, configuration raft.Configuration, configurationIndex uint64, trans raft.Transport) (raft.SnapshotSink, error) {
	// We only support version 1 snapshots at this time.
	if version != 1 {
		return nil, fmt.Errorf("unsupported snapshot version %d", version)
	}

	// Create the sink
	sink := &S3SnapshotSink{
		store:  f,
		logger: f.logger,
		meta: raft.SnapshotMeta{
			Version:            version,
			ID:                 snapshotName(term, index),
			Index:              index,
			Term:               term,
			Configuration:      configuration,
			ConfigurationIndex: configurationIndex,
		},
		trans: trans,
	}

	return sink, nil
}

// List returns available snapshots in the store. It only returns s3
// snapshots. No snapshot will be returned if there are no indexes in the
// FSM.
func (f *S3SnapshotStore) List() ([]*raft.SnapshotMeta, error) {

	//TODO: Read this from S3 by prefix:

	// list files by prefix

	meta, err := f.getMetaFromFSM()
	if err != nil {
		return nil, err
	}

	// If we haven't seen any data yet do not return a snapshot
	if meta.Index == 0 {
		return nil, nil
	}

	return []*raft.SnapshotMeta{meta}, nil
}

// getS3SnapshotMeta returns the fsm's latest state and configuration.
func (f *S3SnapshotStore) getMetaFromSnapshotName(name string) (*raft.SnapshotMeta, error) {

	// get meta (saved file or from file name)
	sections := strings.Split(name, "-")
	termStr := sections[0]
	indexStr := sections[1]

	term, err := strconv.ParseInt(termStr, 10, 64)
	if err != nil {
		// TODO: Log error
		return nil, err
	}

	index, err := strconv.ParseInt(indexStr, 10, 64)
	if err != nil {
		// TODO: Log error
		return nil, err
	}

	//latestIndex, latestConfig := f.fsm.LatestState()
	meta := &raft.SnapshotMeta{
		Version: 1,
		ID:      name,
		Index:   uint64(index),
		Term:    uint64(term),
	}

	raft.GetConfiguration(f.config, *f.fsm, f.lo)


	//	func GetConfiguration(conf *Config, fsm FSM, logs LogStore, stable StableStore,
	//	snaps SnapshotStore, trans Transport) (Configuration, error)


	//	if latestConfig != nil {
	//		meta.ConfigurationIndex, meta.Configuration = protoConfigurationToRaftConfiguration(latestConfig)
	//	}

	return meta, nil
}

// Open takes a snapshot ID and returns a ReadCloser for that snapshot.
func (f *S3SnapshotStore) Open(id string) (*raft.SnapshotMeta, io.ReadCloser, error) {
	if id == s3SnapshotID {
		return f.openFromFSM()
	}

	return f.openFromFile(id)
}

func (f *S3SnapshotStore) openFromFSM() (*raft.SnapshotMeta, io.ReadCloser, error) {
	meta, err := f.getMetaFromFSM()
	if err != nil {
		return nil, nil, err
	}
	// If we don't have any data return an error
	if meta.Index == 0 {
		return nil, nil, errors.New("no snapshot data")
	}

	// Stream data out of the FSM to calculate the size
	readCloser, writeCloser := io.Pipe()
	metaReadCloser, metaWriteCloser := io.Pipe()
	go func() {
		f.fsm.writeTo(context.Background(), metaWriteCloser, writeCloser)
	}()

	// Compute the size
	n, err := io.Copy(ioutil.Discard, metaReadCloser)
	if err != nil {
		f.logger.Error("failed to read state file", "error", err)
		metaReadCloser.Close()
		readCloser.Close()
		return nil, nil, err
	}

	meta.Size = n
	metaReadCloser.Close()

	return meta, readCloser, nil
}

func (f *S3SnapshotStore) openFromFile(id string) (*raft.SnapshotMeta, io.ReadCloser, error) {
	meta, err := f.getMetaFromDB(id)
	if err != nil {
		return nil, nil, err
	}

	filename := filepath.Join(f.path, id, databaseFilename)
	installer := &s3SnapshotInstaller{
		meta:       meta,
		ReadCloser: ioutil.NopCloser(strings.NewReader(filename)),
		filename:   filename,
	}

	return meta, installer, nil
}

// ReapSnapshots reaps all snapshots.
func (f *S3SnapshotStore) ReapSnapshots() error {
	snapshots, err := ioutil.ReadDir(f.path)
	switch {
	case err == nil:
	case os.IsNotExist(err):
		return nil
	default:
		f.logger.Error("failed to scan snapshot directory", "error", err)
		return err
	}

	for _, snap := range snapshots {
		// Ignore any files
		if !snap.IsDir() {
			continue
		}

		// Warn about temporary snapshots, this indicates a previously failed
		// snapshot attempt. We still want to clean these up.
		dirName := snap.Name()
		if strings.HasSuffix(dirName, tmpSuffix) {
			f.logger.Warn("found temporary snapshot", "name", dirName)
		}

		path := filepath.Join(f.path, dirName)
		f.logger.Info("reaping snapshot", "path", path)
		if err := os.RemoveAll(path); err != nil {
			f.logger.Error("failed to reap snapshot", "path", snap.Name(), "error", err)
			return err
		}
	}

	return nil
}

// ID returns the ID of the snapshot, can be used with Open()
// after the snapshot is finalized.
func (s *S3SnapshotSink) ID() string {
	s.l.Lock()
	defer s.l.Unlock()

	return s.meta.ID
}

func (s *S3SnapshotSink) writeS3DBFile() error {
	// Create a new path
	name := snapshotName(s.meta.Term, s.meta.Index)
	path := filepath.Join(s.store.path, name+tmpSuffix)
	s.logger.Info("creating new snapshot", "path", path)

	// Make the directory
	if err := os.MkdirAll(path, 0o755); err != nil {
		s.logger.Error("failed to make snapshot directory", "error", err)
		return err
	}

	dbPath := filepath.Join(path, databaseFilename)

	// Write the snapshot metadata
	if err := writeSnapshotMetaToDB(&s.meta, s3); err != nil {
		return err
	}

	// Set the snapshot ID to the generated name.
	s.meta.ID = name

	// Create the done channel
	s.doneWritingCh = make(chan struct{})

	// Store the directories so we can commit the changes on success or abort
	// them on failure.
	s.dir = path
	s.parentDir = s.store.path

	// Create a pipe so we pipe writes into the go routine below.
	reader, writer := io.Pipe()
	s.writer = writer

	// Start a go routine in charge of piping data from the snapshot's Write
	// call to the delimtedreader and the S3DB file.
	go func() {
		defer close(s.doneWritingCh)
		defer s3.Close()

		// The delimted reader will parse full proto messages from the snapshot
		// data.
		protoReader := NewDelimitedReader(reader, math.MaxInt32)
		defer protoReader.Close()

		var done bool
		var keys int
		entry := new(pb.StorageEntry)
		for !done {
			err := s3.Update(func(tx *s3.Tx) error {
				b, err := tx.CreateBucketIfNotExists(dataBucketName)
				if err != nil {
					return err
				}

				// Commit in batches of 50k. S3 holds all the data in memory and
				// doesn't split the pages until commit so we do incremental writes.
				for i := 0; i < 50000; i++ {
					err := protoReader.ReadMsg(entry)
					if err != nil {
						if err == io.EOF {
							done = true
							return nil
						}
						return err
					}

					err = b.Put([]byte(entry.Key), entry.Value)
					if err != nil {
						return err
					}
					keys += 1
				}

				return nil
			})
			if err != nil {
				s.logger.Error("snapshot write: failed to write transaction", "error", err)
				s.writeError = err
				return
			}

			s.logger.Trace("snapshot write: writing keys", "num_written", keys)
		}
	}()

	return nil
}

// Write is used to append to the s3 file. The first call to write ensures we
// have the file created.
func (s *S3SnapshotSink) Write(b []byte) (int, error) {
	s.l.Lock()
	defer s.l.Unlock()

	// If this is the first call to Write we need to setup the s3 file and
	// kickoff the pipeline write
	if previouslyWritten := s.written.Swap(true); !previouslyWritten {
		// Reap any old snapshots
		if err := s.store.ReapSnapshots(); err != nil {
			return 0, err
		}

		if err := s.writeS3DBFile(); err != nil {
			return 0, err
		}
	}

	return s.writer.Write(b)
}

func (s *S3SnapshotSink) writeSnapshotToS3(b []byte) (int, error) {
	s.l.Lock()
	defer s.l.Unlock()

	// If this is the first call to Write we need to setup the s3 file and
	// kickoff the pipeline write
	if previouslyWritten := s.written.Swap(true); !previouslyWritten {
		// Reap any old snapshots
		if err := s.store.ReapSnapshots(); err != nil {
			return 0, err
		}

		if err := s.writeS3DBFile(); err != nil {
			return 0, err
		}
	}

	return s.writer.Write(b)
}

// Close is used to indicate a successful end.
func (s *S3SnapshotSink) Close() error {

	s.l.Lock()
	defer s.l.Unlock()

	// Make sure close is idempotent
	if s.closed {
		return nil
	}
	s.closed = true

	if s.writer != nil {

		s.writer.Close()
		<-s.doneWritingCh

		if s.writeError != nil {
			// If we encountered an error while writing then we should remove
			// the directory and return the error
			_ = os.RemoveAll(s.dir)
			return s.writeError
		}

		// Move the directory into place
		newPath := strings.TrimSuffix(s.dir, tmpSuffix)

		var err error
		if runtime.GOOS != "windows" {
			err = safeio.Rename(s.dir, newPath)
		} else {
			err = os.Rename(s.dir, newPath)
		}

		if err != nil {
			s.logger.Error("failed to move snapshot into place", "error", err)
			return err
		}
	}

	return nil
}

// Cancel is used to indicate an unsuccessful end.
func (s *S3SnapshotSink) Cancel() error {
	s.l.Lock()
	defer s.l.Unlock()

	// Make sure close is idempotent
	if s.closed {
		return nil
	}
	s.closed = true

	if s.writer != nil {
		s.writer.Close()
		<-s.doneWritingCh

		// Attempt to remove all artifacts
		return os.RemoveAll(s.dir)
	}

	return nil
}

type s3SnapshotInstaller struct {
	io.ReadCloser
	meta     *raft.SnapshotMeta
	filename string
}

func (i *s3SnapshotInstaller) Filename() string {
	return i.filename
}

func (i *s3SnapshotInstaller) Metadata() *raft.SnapshotMeta {
	return i.meta
}

func (i *s3SnapshotInstaller) Install(filename string) error {
	if len(i.filename) == 0 {
		return errors.New("snapshot filename empty")
	}

	if len(filename) == 0 {
		return errors.New("fsm filename empty")
	}

	// Rename the snapshot to the FSM location
	if runtime.GOOS != "windows" {
		return safeio.Rename(i.filename, filename)
	} else {
		return os.Rename(i.filename, filename)
	}
}

// snapshotName generates a name for the snapshot.
func snapshotName(term, index uint64) string {
	now := time.Now()
	msec := now.UnixNano() / int64(time.Millisecond)
	return fmt.Sprintf("%d-%d-%d", term, index, msec)
}
*/
