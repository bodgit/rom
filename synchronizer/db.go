package synchronizer

import (
	"sync"

	"github.com/bodgit/rom"
)

type source struct {
	Name string
	File string
}

// DB holds a collection of ROM checksums and the file(s) that provides them
type DB struct {
	checksums map[checksum][]source
	mutex     sync.Mutex
}

func newDB() (*DB, error) {
	return &DB{
		checksums: make(map[checksum][]source),
	}, nil
}

func (db *DB) scan(reader rom.Reader, t rom.Checksum) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	for _, file := range reader.Files() {
		size, header, err := reader.Size(file)
		if err != nil {
			return err
		}

		c, err := reader.Checksum(file, t)
		if err != nil {
			return err
		}

		checksum := checksum{
			Type:  t,
			Value: checksumToString(c),
			Size:  size - header,
		}

		db.checksums[checksum] = append(db.checksums[checksum], source{reader.Name(), file})
	}

	return nil
}

func (db *DB) find(checksum checksum) []source {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	return db.checksums[checksum]
}

func (db *DB) invalidate(name string) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	for k, v := range db.checksums {
		tmp := v[:0]
		for _, s := range v {
			if name != s.Name {
				tmp = append(tmp, s)
			}
		}
		if len(tmp) == 0 {
			delete(db.checksums, k)
			continue
		}
		db.checksums[k] = tmp
	}
}
