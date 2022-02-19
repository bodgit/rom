package synchronizer

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/bodgit/rom"
	"github.com/bodgit/rom/dat"
)

func (s *Synchronizer) findFiles(ctx context.Context, dir string) (<-chan string, <-chan error, error) {
	out := make(chan string)
	errc := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errc)
		errc <- filepath.Walk(dir, func(file string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Ignore any hidden files or directories, otherwise we end up fighting with things like Spotlight, etc.
			if info.Name()[0] == '.' && (info.Mode().IsDir() || strings.HasPrefix(info.Name(), "._")) {
				s.logger.Println("Ignoring", filepath.Join(dir, info.Name()))
				if info.Mode().IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Ignore anything that isn't a normal file
			if !info.Mode().IsRegular() {
				return nil
			}

			select {
			case out <- file:
			case <-ctx.Done():
				return errors.New("walk cancelled")
			}

			return nil
		})
	}()
	return out, errc, nil
}

func (s *Synchronizer) mergeFiles(ctx context.Context, in ...<-chan string) (<-chan string, <-chan error, error) {
	var wg sync.WaitGroup
	out := make(chan string)
	errc := make(chan error, 1)
	wg.Add(len(in))
	for _, c := range in {
		go func(c <-chan string) {
			defer wg.Done()
			for n := range c {
				select {
				case out <- n:
				case <-ctx.Done():
					return
				}
			}
		}(c)
	}
	go func() {
		wg.Wait()
		close(out)
		close(errc)
	}()
	return out, errc, nil
}

func (s *Synchronizer) scanFiles(ctx context.Context, db *DB, in <-chan string) (<-chan error, error) {
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		for file := range in {
			reader, err := rom.NewReader(file)
			if err != nil {
				errc <- err
				return
			}
			defer reader.Close()

			s.logger.Println("Scanning", reader.Name())

			if err = db.scan(reader, s.checksum); err != nil {
				errc <- err
				return
			}

			reader.Close()
			atomic.AddUint64(&s.rx, reader.Rx())
		}
	}()
	return errc, nil
}

func (s *Synchronizer) allGames(ctx context.Context, datfile *dat.File) (<-chan dat.Game, <-chan error) {
	out := make(chan dat.Game)
	errc := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errc)
		for _, game := range datfile.Game {
			if _, ok := s.missing[game.Name]; ok {
				s.logger.Println("Skipping", game.Name)
				game.Matched()
				continue
			}
			select {
			case out <- game:
			case <-ctx.Done():
				errc <- errors.New("cancelled")
			}
		}
	}()
	return out, errc
}

func popularSource(sources map[string][]source) string {
	m := make(map[string]int)
	for _, v := range sources {
		if len(v) > 1 {
			for _, s := range v {
				m[s.Name]++
			}
		}
	}

	// All ROMs have just the one source
	if len(m) == 0 {
		return ""
	}

	type kv struct {
		k string
		v int
	}

	var ss []kv
	for k, v := range m {
		ss = append(ss, kv{k, v})
	}

	sort.Slice(ss, func(i, j int) bool {
		return ss[i].v > ss[j].v
	})

	return ss[0].k
}

func (s *Synchronizer) transfer(writer rom.Writer, game dat.Game, sources map[string][]source) error {
	// Reduce the sources down to the fewest that provide the most
	for name := popularSource(sources); name != ""; name = popularSource(sources) {
		for k, v := range sources {
			if len(v) == 1 {
				continue
			}
			for _, s := range v {
				if name == s.Name {
					sources[k] = []source{s}
					break
				}
			}
		}
	}

	readers := make(map[string]rom.Reader)

	for _, r := range game.ROM {
		source, ok := sources[r.Name]
		if !ok {
			continue
		}

		src := source[0]

		reader, ok := readers[src.Name]
		if !ok {
			var err error
			if reader, err = rom.NewReader(src.Name); err != nil {
				return err
			}
			defer reader.Close()
			readers[src.Name] = reader
		}

		rr, err := reader.Open(src.File)
		if err != nil {
			return err
		}
		defer rr.Close()

		rw, err := writer.Create(r.Name)
		if err != nil {
			return err
		}
		defer rw.Close()

		s.logger.Println("Copying", src.File, "from", reader.Name(), "to", writer.Name(), "as", r.Name)

		if _, err = io.Copy(rw, rr); err != nil {
			return err
		}

		rw.Close()
		rr.Close()
	}

	for _, reader := range readers {
		reader.Close()
		atomic.AddUint64(&s.rx, reader.Rx())
	}

	return nil
}

func (s *Synchronizer) create(game dat.Game, dir string, db *DB) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	sources := make(map[string][]source, len(game.ROM))

	for _, r := range game.ROM {
		if s := db.find(romChecksum(r, s.checksum)); s != nil && len(s) > 0 {
			sources[r.Name] = s
		}
	}

	if len(sources) == 0 {
		return nil
	}

	s.logger.Println("Creating", gameFilename(game))

	if s.dryRun {
		return nil
	}

	writer, err := rom.NewTorrentZipWriter(filepath.Join(dir, gameFilename(game)))
	if err != nil {
		return err
	}
	defer writer.Close()

	if err := s.transfer(writer, game, sources); err != nil {
		return err
	}

	writer.Close()
	atomic.AddUint64(&s.tx, writer.Tx())

	reader, err := rom.NewTorrentZipReader(filepath.Join(dir, gameFilename(game)))
	if err != nil {
		return err
	}
	defer reader.Close()

	if err = db.scan(reader, s.checksum); err != nil {
		return err
	}

	reader.Close()
	atomic.AddUint64(&s.rx, reader.Rx())

	return nil
}

func (s *Synchronizer) modify(game dat.Game, dir string, db *DB) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var reader rom.Reader
	var err error

	rewrite := false

	if reader, err = rom.NewTorrentZipReader(filepath.Join(dir, gameFilename(game))); err != nil {
		if err != rom.ErrNotTorrentZip {
			return err
		}

		if reader, err = rom.NewZipReader(filepath.Join(dir, gameFilename(game))); err != nil {
			return err
		}
	}
	defer reader.Close()

	if v, ok := reader.(rom.Validator); !ok || !v.Valid() {
		rewrite = true
	}

	sources := make(map[string][]source, len(game.ROM))

rom:
	for _, r := range game.ROM {
		if srcs := db.find(romChecksum(r, s.checksum)); srcs != nil && len(srcs) > 0 {
			for _, src := range srcs {
				if src.Name == reader.Name() && src.File == r.Name {
					sources[r.Name] = []source{{reader.Name(), r.Name}}
					continue rom
				}
			}

			rewrite = true
			sources[r.Name] = srcs
		}
	}

	reader.Close()
	atomic.AddUint64(&s.rx, reader.Rx())

	if !rewrite && len(sources) == len(reader.Files()) {
		return nil
	}

	switch len(sources) {
	case 0:
		s.logger.Println("Deleting", reader.Name())
		if s.dryRun {
			return nil
		}
		return os.RemoveAll(reader.Name())
	case len(reader.Files()):
		s.logger.Println("Rebuilding", reader.Name())
	default:
		s.logger.Println("Modifying", reader.Name())
	}

	if s.dryRun {
		return nil
	}

	temp, err := ioutil.TempDir(dir, "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)

	filename := filepath.Join(temp, gameFilename(game))
	writer, err := rom.NewTorrentZipWriter(filename)
	if err != nil {
		return err
	}
	defer writer.Close()

	if err := s.transfer(writer, game, sources); err != nil {
		return err
	}

	writer.Close()
	atomic.AddUint64(&s.tx, writer.Tx())

	if err := os.Rename(filename, reader.Name()); err != nil {
		return err
	}

	db.invalidate(reader.Name())

	reader, err = rom.NewTorrentZipReader(filepath.Join(dir, gameFilename(game)))
	if err != nil {
		return err
	}
	defer reader.Close()

	if err = db.scan(reader, s.checksum); err != nil {
		return err
	}

	reader.Close()
	atomic.AddUint64(&s.rx, reader.Rx())

	return nil
}

func (s *Synchronizer) gameWorker(ctx context.Context, dir string, datfile *dat.File, db *DB, in <-chan dat.Game) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		for game := range in {
			if reader, err := rom.NewZipReader(filepath.Join(dir, gameFilename(game))); err != nil {
				if !os.IsNotExist(err) {
					errc <- err
					return
				}
				if err := s.create(game, dir, db); err != nil {
					errc <- err
					return
				}
			} else {
				reader.Close()
				atomic.AddUint64(&s.rx, reader.Rx())

				if err := s.modify(game, dir, db); err != nil {
					errc <- err
					return
				}
			}

			reader, err := rom.NewZipReader(filepath.Join(dir, gameFilename(game)))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				errc <- err
				return
			}

			files := reader.Files()
			sort.Strings(files)

			for i, r := range game.ROM {
				if j := sort.SearchStrings(files, r.Name); j < len(files) && files[j] == r.Name {
					game.ROM[i].Matched()
				}
			}

			reader.Close()
			atomic.AddUint64(&s.rx, reader.Rx())
		}
	}()
	return errc
}

func waitForPipeline(errs ...<-chan error) error {
	errc := mergeErrors(errs...)
	for err := range errc {
		if err != nil {
			return err
		}
	}
	return nil
}

func mergeErrors(cs ...<-chan error) <-chan error {
	var wg sync.WaitGroup
	out := make(chan error, len(cs))
	wg.Add(len(cs))
	for _, c := range cs {
		go func(c <-chan error) {
			for n := range c {
				out <- n
			}
			wg.Done()
		}(c)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}
