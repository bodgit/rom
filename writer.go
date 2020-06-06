package rom

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/bodgit/plumbing"
	"github.com/uwedeportivo/torrentzip"
)

// Writer is the interface implemented by all ROM writers
type Writer interface {
	// Close closes access to the underlying file. Any other methods
	// are not guaranteed to work after this has been called
	Close() error
	// Create returns an io.WriteCloser for the requested filename. The
	// ability to create multiple files in parallel rather than
	// sequentially is implementation-dependent
	Create(string) (io.WriteCloser, error)
	// Name returns the full path to the underlying file
	Name() string
	// Tx returns the number of bytes written by the implementation
	Tx() uint64
}

var errDirectoryNotSupported = errors.New("directories not supported")

// FileWriter writes a single regular file as if it was an archive
// containing exactly one file. The one file must match the base name of
// the target
type FileWriter struct {
	filename string
	tx       plumbing.WriteCounter
}

// NewFileWriter returns a new FileWriter for the passed filename
func NewFileWriter(filename string) (*FileWriter, error) {
	if err := os.RemoveAll(filename); err != nil {
		return nil, err
	}

	return &FileWriter{
		filename: filename,
	}, nil
}

// Close closes access to the underlying file. Any other methods are not
// guaranteed to work after this has been called
func (w *FileWriter) Close() error {
	return nil
}

// Create returns an io.WriteCloser for the requested filename. The ability
// to create multiple files in parallel rather than sequentially is
// implementation-dependent
func (w *FileWriter) Create(filename string) (io.WriteCloser, error) {
	if filename != filepath.Base(w.filename) {
		return nil, errDirectoryNotSupported
	}
	writer, err := os.Create(w.filename)
	if err != nil {
		return nil, err
	}
	return plumbing.MultiWriteCloser(writer, plumbing.NopWriteCloser(&w.tx)), nil
}

// Name returns the full path to the underlying file
func (w *FileWriter) Name() string {
	return w.filename
}

// Tx returns the number of bytes written by the implementation
func (w *FileWriter) Tx() uint64 {
	return w.tx.Count()
}

// DirectoryWriter creates a directory if necessary and then writes new
// files inside it
type DirectoryWriter struct {
	directory string
	tx        plumbing.WriteCounter
}

// NewDirectoryWriter returns a new DirectoryWriter for the passed
// directory. Any existing files within it are removed
func NewDirectoryWriter(directory string) (*DirectoryWriter, error) {
	if err := os.MkdirAll(directory, os.ModePerm); err != nil {
		return nil, err
	}

	dir, err := os.Open(directory)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	names, err := dir.Readdirnames(0)
	if err != nil {
		return nil, err
	}

	for _, name := range names {
		if err := os.RemoveAll(filepath.Join(directory, name)); err != nil {
			return nil, err
		}
	}

	return &DirectoryWriter{
		directory: directory,
	}, nil
}

// Close closes access to the underlying file. Any other methods are not
// guaranteed to work after this has been called
func (w *DirectoryWriter) Close() error {
	return nil
}

// Create returns an io.WriteCloser for the requested filename. The ability
// to create multiple files in parallel rather than sequentially is
// implementation-dependent
func (w *DirectoryWriter) Create(filename string) (io.WriteCloser, error) {
	if filename != filepath.Base(filename) {
		return nil, errDirectoryNotSupported
	}
	writer, err := os.Create(filepath.Join(w.directory, filename))
	if err != nil {
		return nil, err
	}
	return plumbing.MultiWriteCloser(writer, plumbing.NopWriteCloser(&w.tx)), nil
}

// Name returns the full path to the underlying file
func (w *DirectoryWriter) Name() string {
	return w.directory
}

// Tx returns the number of bytes written by the implementation
func (w *DirectoryWriter) Tx() uint64 {
	return w.tx.Count()
}

// ZipWriter creates a new zip archive
type ZipWriter struct {
	file   *os.File
	writer *zip.Writer
	tx     plumbing.WriteCounter
}

// NewZipWriter returns a new ZipWriter for the passed zip archive
func NewZipWriter(filename string) (*ZipWriter, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	w := &ZipWriter{
		file: file,
	}

	w.writer = zip.NewWriter(io.MultiWriter(file, &w.tx))

	return w, nil
}

// Close closes access to the underlying file. Any other methods are not
// guaranteed to work after this has been called
func (w *ZipWriter) Close() error {
	if err := w.writer.Close(); err != nil {
		return err
	}

	return w.file.Close()
}

// Create returns an io.WriteCloser for the requested filename. The ability
// to create multiple files in parallel rather than sequentially is
// implementation-dependent
func (w *ZipWriter) Create(filename string) (io.WriteCloser, error) {
	if filename != filepath.Base(filename) {
		return nil, errDirectoryNotSupported
	}
	writer, err := w.writer.Create(filename)
	if err != nil {
		return nil, err
	}
	return plumbing.NopWriteCloser(writer), nil
}

// Name returns the full path to the underlying file
func (w *ZipWriter) Name() string {
	return w.file.Name()
}

// Tx returns the number of bytes written by the implementation
func (w *ZipWriter) Tx() uint64 {
	return w.tx.Count()
}

// TorrentZipWriter creates a new zip archive using the torrentzip
// standard. It is slightly slower to create than a normal zip archive
type TorrentZipWriter struct {
	file   *os.File
	writer *torrentzip.Writer
	tx     plumbing.WriteCounter
}

// NewTorrentZipWriter returns a new TorrentZipWriter for the passed zip
// archive
func NewTorrentZipWriter(filename string) (*TorrentZipWriter, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	w := &TorrentZipWriter{
		file: file,
	}

	// Try and keep the temporary file on the same filesystem as the target file
	w.writer, err = torrentzip.NewWriterWithTemp(io.MultiWriter(file, &w.tx), filepath.Dir(filename))
	if err != nil {
		return nil, err
	}

	return w, nil
}

// Close closes access to the underlying file. Any other methods are not
// guaranteed to work after this has been called
func (w *TorrentZipWriter) Close() error {
	if err := w.writer.Close(); err != nil {
		return err
	}

	return w.file.Close()
}

// Create returns an io.WriteCloser for the requested filename. The ability
// to create multiple files in parallel rather than sequentially is
// implementation-dependent
func (w *TorrentZipWriter) Create(filename string) (io.WriteCloser, error) {
	writer, err := w.writer.Create(filename)
	if err != nil {
		return nil, err
	}
	return plumbing.NopWriteCloser(writer), nil
}

// Name returns the full path to the underlying file
func (w *TorrentZipWriter) Name() string {
	return w.file.Name()
}

// BUG(bodgit): The bytes written for TorrentZipWriter is not accurate

// Tx returns the number of bytes written by the implementation
func (w *TorrentZipWriter) Tx() uint64 {
	return w.tx.Count()
}
