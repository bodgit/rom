package rom

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewReader(t *testing.T) {
	tables := map[string]struct {
		path   string
		err    error
		reader string
		files  []string
	}{
		"file": {
			filepath.Join("testdata", "test", "test.bin"),
			nil,
			"*rom.FileReader",
			[]string{"test.bin"},
		},
		"directory": {
			filepath.Join("testdata", "test"),
			nil,
			"*rom.DirectoryReader",
			[]string{"test.bin", "test.nes"},
		},
		"zip": {
			filepath.Join("testdata", "test.zip"),
			nil,
			"*rom.ZipReader",
			[]string{"test.bin", "test.nes"},
		},
		"torrentzip": {
			filepath.Join("testdata", "torrent.zip"),
			nil,
			"*rom.TorrentZipReader",
			[]string{"test.bin", "test.nes"},
		},
		"7z": {
			filepath.Join("testdata", "test.7z"),
			nil,
			"*rom.SevenZipReader",
			[]string{"test.bin", "test.nes"},
		},
		"rar": {
			filepath.Join("testdata", "test.rar"),
			nil,
			"*rom.RarReader",
			[]string{"test.bin", "test.nes"},
		},
		"nonexistent": {
			filepath.Join("testdata", "error"),
			&os.PathError{
				Op:   "stat",
				Path: filepath.Join("testdata", "error"),
				Err:  syscall.ENOENT,
			},
			"",
			[]string{},
		},
	}

	for name, table := range tables {
		t.Run(name, func(t *testing.T) {
			r, err := NewReader(table.path)
			assert.Equal(t, table.err, err)
			if err == nil {
				assert.Equal(t, table.reader, fmt.Sprintf("%T", r))
				assert.Equal(t, table.path, r.Name())
				files := r.Files()
				sort.Strings(files)
				assert.Equal(t, table.files, files)

				_, _, err = r.Size("nonexistent")
				assert.Equal(t, errFileNotFound, err)

				size, header, err := r.Size("test.bin")
				assert.Equal(t, nil, err)
				assert.Equal(t, uint64(20), size)
				assert.Equal(t, uint64(0), header)

				_, err = r.Checksum("nonexistent", MD5)
				assert.Equal(t, errFileNotFound, err)

				checksum, err := r.Checksum("test.bin", CRC32)
				assert.Equal(t, nil, err)
				assert.Equal(t, []byte{0xd5, 0x80, 0xa1, 0x53}, checksum)

				checksum, err = r.Checksum("test.bin", SHA1)
				assert.Equal(t, nil, err)
				assert.Equal(t, []byte{0x4e, 0xbc, 0x20, 0xb4, 0x6e, 0xa4, 0xd0, 0x10, 0xed, 0x9a, 0xc1, 0xfd, 0xe4, 0xc2, 0x51, 0xcf, 0x23, 0x1a, 0x66, 0x1f}, checksum)

				_, err = r.Open("nonexistent")
				assert.Equal(t, errFileNotFound, err)

				reader, err := r.Open("test.bin")
				assert.Equal(t, nil, err)
				b := new(bytes.Buffer)
				if n, err := io.Copy(b, reader); uint64(n) != size || err != nil {
					if err != nil {
						t.Fatal(err)
					}
					t.Fatal(errors.New("not read enough"))
				}
				assert.Equal(t, []byte{0xca, 0xc6, 0x80, 0x38, 0xd6, 0x93, 0xcb, 0x64, 0x5b, 0x85, 0xa9, 0x99, 0x05, 0x20, 0xbc, 0x74, 0xdd, 0x96, 0x53, 0xb7}, b.Bytes())
				assert.Equal(t, nil, reader.Close())

				if v, ok := r.(Validator); ok {
					assert.Equal(t, true, v.Valid())
				}

				assert.Equal(t, nil, r.Close())
				assert.Greater(t, r.Rx(), uint64(0))
			}
		})
	}
}
