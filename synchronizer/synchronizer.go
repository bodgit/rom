/*
Package synchronizer implements a set of methods to maintain a pristine
directory of TorrentZip files representing the games in a dat file.
*/
package synchronizer

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/bodgit/rom"
	"github.com/bodgit/rom/dat"
)

// Synchronizer encapsulates the configuration
type Synchronizer struct {
	mutex    sync.RWMutex
	workers  int
	dryRun   bool
	checksum rom.Checksum
	logger   *log.Logger
	rx       uint64
	tx       uint64
	missing  map[string]struct{}
}

// NewSynchronizer returns a new Synchronizer configured with any optional
// settings
func NewSynchronizer(options ...func(*Synchronizer) error) (*Synchronizer, error) {
	s := new(Synchronizer)

	s.logger = log.New(os.Stderr, "", log.LstdFlags)

	if err := s.setOption(options...); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Synchronizer) setOption(options ...func(*Synchronizer) error) error {
	for _, option := range options {
		if err := option(s); err != nil {
			return err
		}
	}
	return nil
}

// Workers sets the numbers of workers used
func Workers(count int) func(*Synchronizer) error {
	return func(s *Synchronizer) error {
		s.workers = count
		return nil
	}
}

// SetWorkers sets the number of workers used by s
func (s *Synchronizer) SetWorkers(count int) error {
	return s.setOption(Workers(count))
}

// DryRun configures whether changes are only logged
func DryRun(v bool) func(*Synchronizer) error {
	return func(s *Synchronizer) error {
		s.dryRun = v
		return nil
	}
}

// SetDryRun configures whether changes are only logged by s
func (s *Synchronizer) SetDryRun(v bool) error {
	return s.setOption(DryRun(v))
}

// Logger configures the logger used
func Logger(logger *log.Logger) func(*Synchronizer) error {
	return func(s *Synchronizer) error {
		s.logger = logger
		return nil
	}
}

// SetLogger configures the logger used by s
func (s *Synchronizer) SetLogger(logger *log.Logger) error {
	return s.setOption(Logger(logger))
}

// Checksum configures the checksum algorithm used
func Checksum(c rom.Checksum) func(*Synchronizer) error {
	return func(s *Synchronizer) error {
		s.checksum = c
		return nil
	}
}

// SetChecksum configures the checksum algorithm used by s
func (s *Synchronizer) SetChecksum(c rom.Checksum) error {
	return s.setOption(Checksum(c))
}

// Missing reads from r a list of missing games
func Missing(r io.Reader) func(*Synchronizer) error {
	return func(s *Synchronizer) error {
		scanner := bufio.NewScanner(r)
		s.missing = make(map[string]struct{})
		for scanner.Scan() {
			s.missing[scanner.Text()] = struct{}{}
		}
		return scanner.Err()
	}
}

// SetMissing reads from r a list of missing games
func (s *Synchronizer) SetMissing(r io.Reader) error {
	return s.setOption(Missing(r))
}

// Scan reads one or more directories and any archives within and stores the
// checksum of every file using the chosen checksum algorithm
func (s *Synchronizer) Scan(dirs ...string) (*DB, error) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	var filecList []<-chan string
	var errcList []<-chan error

	for _, dir := range dirs {
		filec, errc, err := s.findFiles(ctx, dir)
		if err != nil {
			return nil, err
		}
		filecList = append(filecList, filec)
		errcList = append(errcList, errc)
	}

	mergec, errc, err := s.mergeFiles(ctx, filecList...)
	if err != nil {
		return nil, err
	}
	errcList = append(errcList, errc)

	db, err := newDB()
	if err != nil {
		return nil, err
	}

	workers := s.workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	for i := 0; i < workers; i++ {
		errc, err := s.scanFiles(ctx, db, mergec)
		if err != nil {
			return nil, err
		}
		errcList = append(errcList, errc)
	}

	if err := waitForPipeline(errcList...); err != nil {
		return nil, err
	}

	return db, nil
}

// Update attempts to keep dir synchronized with the provided datfile using
// db to find any missing files based on the checksum value
func (s *Synchronizer) Update(dir string, datfile *dat.File, db *DB) error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	var errcList []<-chan error

	gamec, errc := s.allGames(ctx, datfile)
	errcList = append(errcList, errc)

	workers := s.workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	for i := 0; i < workers; i++ {
		errc := s.gameWorker(ctx, dir, datfile, db, gamec)
		errcList = append(errcList, errc)
	}

	if err := waitForPipeline(errcList...); err != nil {
		return err
	}

	return nil
}

// Delete removes any file from dir that doesn't match a known game
func (s *Synchronizer) Delete(dir string, datfile *dat.File) error {
	games := make(map[string]struct{}, len(datfile.Game))
	for _, game := range datfile.Game {
		games[gameFilename(game)] = struct{}{}
	}

	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()

	files, err := f.Readdirnames(0)
	if err != nil {
		return err
	}

	for _, file := range files {
		if _, ok := games[file]; ok || file[0] == '.' {
			continue
		}
		s.logger.Println("Deleting", file)
		if s.dryRun {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, file)); err != nil {
			return err
		}
	}

	return nil
}

// Reset zeroes the bytes read & written counters
func (s *Synchronizer) Reset() {
	atomic.StoreUint64(&s.rx, 0)
	atomic.StoreUint64(&s.tx, 0)
}

// Rx returns how many bytes have been read by s
func (s *Synchronizer) Rx() uint64 {
	return atomic.LoadUint64(&s.rx)
}

// Tx returns how many bytes have been written by s
func (s *Synchronizer) Tx() uint64 {
	return atomic.LoadUint64(&s.tx)
}
