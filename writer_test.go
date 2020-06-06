package rom

import (
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileWriter(t *testing.T) {
	tables := map[string]struct {
		path string
		err  error
		file string
	}{
		"ok": {
			filepath.Join(os.TempDir(), "test.bin"),
			nil,
			"test.bin",
		},
	}

	for name, table := range tables {
		t.Run(name, func(t *testing.T) {
			w, err := NewFileWriter(table.path)
			assert.Equal(t, table.err, err)
			if err == nil {
				assert.Equal(t, table.path, w.Name())

				writer, err := w.Create(table.file)
				assert.Equal(t, nil, err)
				if n, err := io.CopyN(writer, rand.Reader, 20); n != 20 || err != nil {
					if err != nil {
						t.Fatal(err)
					}
					t.Fatal(errors.New("not read enough"))
				}
				assert.Equal(t, nil, writer.Close())

				assert.Equal(t, nil, w.Close())
				assert.Greater(t, w.Tx(), uint64(0))
				assert.FileExists(t, table.path)
			}
		})
	}
}

func TestDirectoryWriter(t *testing.T) {
	tables := map[string]struct {
		path string
		err  error
		file string
	}{
		"ok": {
			filepath.Join(os.TempDir(), "test"),
			nil,
			"test.bin",
		},
	}

	for name, table := range tables {
		t.Run(name, func(t *testing.T) {
			w, err := NewDirectoryWriter(table.path)
			assert.Equal(t, table.err, err)
			if err == nil {
				assert.Equal(t, table.path, w.Name())

				writer, err := w.Create(table.file)
				assert.Equal(t, nil, err)
				if n, err := io.CopyN(writer, rand.Reader, 20); n != 20 || err != nil {
					if err != nil {
						t.Fatal(err)
					}
					t.Fatal(errors.New("not read enough"))
				}
				assert.Equal(t, nil, writer.Close())

				assert.Equal(t, nil, w.Close())
				assert.Greater(t, w.Tx(), uint64(0))
				assert.DirExists(t, table.path)
				assert.FileExists(t, filepath.Join(table.path, table.file))
			}
		})
	}
}

func TestZipWriter(t *testing.T) {
	tables := map[string]struct {
		path string
		err  error
		file string
	}{
		"ok": {
			filepath.Join(os.TempDir(), "test.zip"),
			nil,
			"test.bin",
		},
	}

	for name, table := range tables {
		t.Run(name, func(t *testing.T) {
			w, err := NewZipWriter(table.path)
			assert.Equal(t, table.err, err)
			if err == nil {
				assert.Equal(t, table.path, w.Name())

				writer, err := w.Create(table.file)
				assert.Equal(t, nil, err)
				if n, err := io.CopyN(writer, rand.Reader, 20); n != 20 || err != nil {
					if err != nil {
						t.Fatal(err)
					}
					t.Fatal(errors.New("not read enough"))
				}
				assert.Equal(t, nil, writer.Close())

				assert.Equal(t, nil, w.Close())
				assert.Greater(t, w.Tx(), uint64(0))
				assert.FileExists(t, table.path)
			}
		})
	}
}

func TestTorrentZipWriter(t *testing.T) {
	tables := map[string]struct {
		path string
		err  error
		file string
	}{
		"ok": {
			filepath.Join(os.TempDir(), "test.zip"),
			nil,
			"test.bin",
		},
	}

	for name, table := range tables {
		t.Run(name, func(t *testing.T) {
			w, err := NewTorrentZipWriter(table.path)
			assert.Equal(t, table.err, err)
			if err == nil {
				assert.Equal(t, table.path, w.Name())

				writer, err := w.Create(table.file)
				assert.Equal(t, nil, err)
				if n, err := io.CopyN(writer, rand.Reader, 20); n != 20 || err != nil {
					if err != nil {
						t.Fatal(err)
					}
					t.Fatal(errors.New("not read enough"))
				}
				assert.Equal(t, nil, writer.Close())

				assert.Equal(t, nil, w.Close())
				assert.Greater(t, w.Tx(), uint64(0))
				assert.FileExists(t, table.path)
			}
		})
	}
}
