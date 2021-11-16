package rom

import (
	"archive/zip"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/plumbing"
	"github.com/bodgit/sevenzip"
	"github.com/gabriel-vasile/mimetype"
)

// Reader is the interface implemented by all ROM readers
type Reader interface {
	// Checksum computes the checksum for the passed file
	Checksum(string, Checksum) ([]byte, error)
	// Close closes access to the underlying file. Any other methods
	// are not guaranteed to work after this has been called
	Close() error
	// Files returns all files accessible by the implementation.
	Files() []string
	// Name returns the full path to the underlying file
	Name() string
	// Open returns an io.ReadCloser for any file listed by the Files
	// method
	Open(string) (io.ReadCloser, error)
	// Rx returns the number of bytes read by the implementation
	Rx() uint64
	// Size returns the size of any file listed by the Files method
	Size(string) (uint64, error)
}

// Validator is the interface optionally implemented by a ROM reader if it can
// validate its integrity somehow
type Validator interface {
	// Valid returns if the underlying file or container is considered
	// correct
	Valid() bool
}

var (
	errNotFile         = errors.New("not a file")
	errNotDirectory    = errors.New("not a directory")
	errFileNotFound    = errors.New("file not found")
	errUnknownChecksum = errors.New("unknown checksum")
	// ErrNotTorrentZip is returned if a zip file does not have the
	// correct archive comment
	ErrNotTorrentZip = errors.New("not a torrent zip")
)

// NewReader uses heuristics to work out the type of file passed and uses
// the most appropriate Reader to access it
func NewReader(path string) (Reader, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return NewDirectoryReader(path)
	}

	mime, err := mimetype.DetectFile(path)
	if err != nil {
		return nil, err
	}

	switch mime.Extension() {
	case ".7z":
		return NewSevenZipReader(path)
	case ".zip":
		r, err := NewTorrentZipReader(path)
		if err != ErrNotTorrentZip {
			return r, err
		}
		return NewZipReader(path)
	}

	return NewFileReader(path)
}

// FileReader reads a single regular file and coerces it into looking like
// an archive containing exactly one file
type FileReader struct {
	checksum  [][]byte
	directory string
	filename  string
	size      uint64
	rx        plumbing.WriteCounter
}

// NewFileReader returns a new FileReader for the passed filename
func NewFileReader(filename string) (*FileReader, error) {
	r := &FileReader{
		directory: filepath.Dir(filename),
		filename:  filepath.Base(filename),
	}

	info, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() {
		return nil, errNotFile
	}

	r.size = uint64(info.Size())

	return r, nil
}

// Checksum computes the checksum for the passed file
func (r *FileReader) Checksum(filename string, checksum Checksum) ([]byte, error) {
	if len(r.checksum) == 0 {
		reader, err := r.Open(filename)
		if err != nil {
			return nil, err
		}
		defer reader.Close()

		if r.checksum, err = checksumFunction(filename)(reader); err != nil {
			return nil, err
		}
	}

	switch checksum {
	case CRC32, MD5, SHA1:
		return r.checksum[checksum], nil
	}

	return nil, errUnknownChecksum
}

// Close closes access to the underlying file. Any other methods are not
// guaranteed to work after this has been called
func (r *FileReader) Close() error {
	return nil
}

// Files returns all files accessible by the implementation.
func (r *FileReader) Files() []string {
	return []string{r.filename}
}

// Name returns the full path to the underlying file
func (r *FileReader) Name() string {
	return filepath.Join(r.directory, r.filename)
}

// Open returns an io.ReadCloser for any file listed by the Files method
func (r *FileReader) Open(filename string) (io.ReadCloser, error) {
	if filename != r.filename {
		return nil, errFileNotFound
	}
	file, err := os.Open(filepath.Join(r.directory, filename))
	if err != nil {
		return nil, err
	}

	return plumbing.TeeReadCloser(file, &r.rx), nil
}

// Rx returns the number of bytes read by the implementation
func (r *FileReader) Rx() uint64 {
	return r.rx.Count()
}

// Size returns the size of any file listed by the Files method
func (r *FileReader) Size(filename string) (uint64, error) {
	if filename != r.filename {
		return 0, errFileNotFound
	}
	return r.size, nil
}

// DirectoryReader reads a directory and provides access to any regular
// files contained within. Hidden files, directories and any files not in
// the immediate directory are inaccessible
type DirectoryReader struct {
	checksums map[string][][]byte
	directory string
	files     map[string]uint64
	rx        plumbing.WriteCounter
}

// NewDirectoryReader returns a new DirectoryReader for the passed
// directory
func NewDirectoryReader(directory string) (*DirectoryReader, error) {
	r := &DirectoryReader{
		checksums: make(map[string][][]byte),
		directory: directory,
		files:     make(map[string]uint64),
	}

	d, err := os.Open(directory)
	if err != nil {
		return nil, err
	}
	defer d.Close()

	info, err := d.Stat()
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, errNotDirectory
	}

	names, err := d.Readdirnames(0)
	if err != nil {
		return nil, err
	}

	for _, name := range names {
		info, err := os.Stat(filepath.Join(directory, name))
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || name[0] == '.' {
			continue
		}
		r.files[name] = uint64(info.Size())
	}

	return r, nil
}

// Checksum computes the checksum for the passed file
func (r *DirectoryReader) Checksum(filename string, checksum Checksum) ([]byte, error) {
	c, ok := r.checksums[filename]
	if !ok {
		reader, err := r.Open(filename)
		if err != nil {
			return nil, err
		}
		defer reader.Close()

		if c, err = checksumFunction(filename)(reader); err != nil {
			return nil, err
		}
		r.checksums[filename] = c
	}

	switch checksum {
	case CRC32, MD5, SHA1:
		return c[checksum], nil
	}

	return nil, errUnknownChecksum
}

// Close closes access to the underlying file. Any other methods are not
// guaranteed to work after this has been called
func (r *DirectoryReader) Close() error {
	return nil
}

// Files returns all files accessible by the implementation.
func (r *DirectoryReader) Files() []string {
	files := []string{}
	for f := range r.files {
		files = append(files, f)
	}
	return files
}

// Name returns the full path to the underlying file
func (r *DirectoryReader) Name() string {
	return r.directory
}

// Open returns an io.ReadCloser for any file listed by the Files method
func (r *DirectoryReader) Open(filename string) (io.ReadCloser, error) {
	if _, ok := r.files[filename]; !ok {
		return nil, errFileNotFound
	}
	file, err := os.Open(filepath.Join(r.directory, filename))
	if err != nil {
		return nil, err
	}
	return plumbing.TeeReadCloser(file, &r.rx), nil
}

// Rx returns the number of bytes read by the implementation
func (r *DirectoryReader) Rx() uint64 {
	return r.rx.Count()
}

// Size returns the size of any file listed by the Files method
func (r *DirectoryReader) Size(filename string) (uint64, error) {
	if size, ok := r.files[filename]; ok {
		return size, nil
	}
	return 0, errFileNotFound
}

// ZipReader reads a zip archive and provides access to any regular files
// contained within. Hidden files, directories and any files not in the
// top level are inaccessible
type ZipReader struct {
	checksums map[string][][]byte
	file      *os.File
	reader    *zip.Reader
	files     map[string]*zip.File
	rx        plumbing.WriteCounter
}

// NewZipReader returns a new ZipReader for the passed zip archive
func NewZipReader(filename string) (r *ZipReader, err error) {
	r = &ZipReader{
		checksums: make(map[string][][]byte),
		files:     make(map[string]*zip.File),
	}

	r.file, err = os.Open(filename)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			r.file.Close()
		}
	}()

	var info os.FileInfo
	info, err = r.file.Stat()
	if err != nil {
		return
	}

	r.reader, err = zip.NewReader(plumbing.TeeReaderAt(r.file, &r.rx), info.Size())
	if err != nil {
		return
	}

	for _, file := range r.reader.File {
		if !file.Mode().IsRegular() || file.Name[0] == '.' || filepath.Dir(file.Name) != "." {
			continue
		}
		r.files[file.Name] = file
	}

	return
}

// Checksum computes the checksum for the passed file. CRC values for files
// that don't have special requirements use the value from the central
// directory
func (r *ZipReader) Checksum(filename string, checksum Checksum) ([]byte, error) {
	file, ok := r.files[filename]
	if !ok {
		return nil, errFileNotFound
	}

	if checksum == CRC32 && !needsDirectChecksum(filename) {
		c := file.CRC32
		return []byte{byte(0xff & (c >> 24)), byte(0xff & (c >> 16)), byte(0xff & (c >> 8)), byte(c)}, nil
	}

	c, ok := r.checksums[filename]
	if !ok {
		reader, err := r.Open(filename)
		if err != nil {
			return nil, err
		}
		defer reader.Close()

		if c, err = checksumFunction(filename)(reader); err != nil {
			return nil, err
		}
		r.checksums[filename] = c
	}

	switch checksum {
	case CRC32, MD5, SHA1:
		return c[checksum], nil
	}

	return nil, errUnknownChecksum
}

// Close closes access to the underlying file. Any other methods are not
// guaranteed to work after this has been called
func (r *ZipReader) Close() error {
	return r.file.Close()
}

// Files returns all files accessible by the implementation.
func (r *ZipReader) Files() []string {
	files := []string{}
	for f := range r.files {
		files = append(files, f)
	}
	return files
}

// Name returns the full path to the underlying file
func (r *ZipReader) Name() string {
	return r.file.Name()
}

// Open returns an io.ReadCloser for any file listed by the Files method
func (r *ZipReader) Open(filename string) (io.ReadCloser, error) {
	file, ok := r.files[filename]
	if !ok {
		return nil, errFileNotFound
	}
	return file.Open()
}

// Rx returns the number of bytes read by the implementation
func (r *ZipReader) Rx() uint64 {
	return r.rx.Count()
}

// Size returns the size of any file listed by the Files method
func (r *ZipReader) Size(filename string) (uint64, error) {
	file, ok := r.files[filename]
	if !ok {
		return 0, errFileNotFound
	}
	return file.UncompressedSize64, nil
}

// TorrentZipReader reads a zip archive and provides access to any regular files
// contained within. Hidden files, directories and any files not in the
// top level are inaccessible
type TorrentZipReader struct {
	*ZipReader
	valid bool
}

const (
	commentPrefix              = "TORRENTZIPPED-"
	localFileHeaderLength      = 30
	centralFileDirectoryLength = 46
)

// NewTorrentZipReader returns a new TorrentZipReader for the passed zip
// archive. It extends NewZipReader to check that the zip archive has the
// correctly formatted comment and validates that the CRC of the central
// directory matches the comment value
func NewTorrentZipReader(filename string) (r *TorrentZipReader, err error) {
	r = new(TorrentZipReader)

	r.ZipReader, err = NewZipReader(filename)
	if err != nil {
		return
	}
	reader := r.ZipReader.reader

	if !strings.HasPrefix(reader.Comment, commentPrefix) {
		err = ErrNotTorrentZip
		return
	}

	// Work out the start and length of the central directory
	socd, eocd := int64(0), int64(0)
	for _, file := range reader.File {
		socd += int64(localFileHeaderLength + len(file.Name))
		socd += int64(file.CompressedSize64)
		eocd += int64(centralFileDirectoryLength + len(file.Name))
	}

	h := crc32.NewIEEE()
	sr := io.NewSectionReader(plumbing.TeeReaderAt(r.ZipReader.file, &r.ZipReader.rx), socd, eocd)
	if _, err = io.Copy(h, sr); err != nil {
		return
	}
	r.valid = strings.TrimPrefix(reader.Comment, commentPrefix) == fmt.Sprintf("%X", h.Sum(nil))

	return
}

// Valid confirms the checksum of the central directory in the zip archive
// matches the value in the archive comment
func (r *TorrentZipReader) Valid() bool {
	return r.valid
}

// SevenZipReader reads a 7zip archive and provides access to any regular
// files contained within. Hidden files, directories and any files not in
// the top level are inaccessible
type SevenZipReader struct {
	checksums map[string][][]byte
	file      *os.File
	reader    *sevenzip.Reader
	files     map[string]*sevenzip.File
	rx        plumbing.WriteCounter
}

// NewSevenZipReader returns a new SevenZipReader for the passed 7zip archive
func NewSevenZipReader(filename string) (r *SevenZipReader, err error) {
	r = &SevenZipReader{
		checksums: make(map[string][][]byte),
		files:     make(map[string]*sevenzip.File),
	}

	r.file, err = os.Open(filename)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			r.file.Close()
		}
	}()

	var info os.FileInfo
	info, err = r.file.Stat()
	if err != nil {
		return
	}

	r.reader, err = sevenzip.NewReader(plumbing.TeeReaderAt(r.file, &r.rx), info.Size())
	if err != nil {
		return
	}

	for _, file := range r.reader.File {
		if !file.Mode().IsRegular() || file.Name[0] == '.' || filepath.Dir(file.Name) != "." {
			continue
		}
		r.files[file.Name] = file
	}

	return
}

// Checksum computes the checksum for the passed file. CRC values for files
// that don't have special requirements use the value from the central
// directory
func (r *SevenZipReader) Checksum(filename string, checksum Checksum) ([]byte, error) {
	file, ok := r.files[filename]
	if !ok {
		return nil, errFileNotFound
	}

	if checksum == CRC32 && !needsDirectChecksum(filename) {
		c := file.CRC32
		return []byte{byte(0xff & (c >> 24)), byte(0xff & (c >> 16)), byte(0xff & (c >> 8)), byte(c)}, nil
	}

	c, ok := r.checksums[filename]
	if !ok {
		reader, err := r.Open(filename)
		if err != nil {
			return nil, err
		}
		defer reader.Close()

		if c, err = checksumFunction(filename)(reader); err != nil {
			return nil, err
		}
		r.checksums[filename] = c
	}

	switch checksum {
	case CRC32, MD5, SHA1:
		return c[checksum], nil
	}

	return nil, errUnknownChecksum
}

// Close closes access to the underlying file. Any other methods are not
// guaranteed to work after this has been called
func (r *SevenZipReader) Close() error {
	return r.file.Close()
}

// Files returns all files accessible by the implementation.
func (r *SevenZipReader) Files() []string {
	files := []string{}
	for f := range r.files {
		files = append(files, f)
	}
	return files
}

// Name returns the full path to the underlying file
func (r *SevenZipReader) Name() string {
	return r.file.Name()
}

// Open returns an io.ReadCloser for any file listed by the Files method
func (r *SevenZipReader) Open(filename string) (io.ReadCloser, error) {
	file, ok := r.files[filename]
	if !ok {
		return nil, errFileNotFound
	}
	return file.Open()
}

// Rx returns the number of bytes read by the implementation
func (r *SevenZipReader) Rx() uint64 {
	return r.rx.Count()
}

// Size returns the size of any file listed by the Files method
func (r *SevenZipReader) Size(filename string) (uint64, error) {
	file, ok := r.files[filename]
	if !ok {
		return 0, errFileNotFound
	}
	return file.UncompressedSize, nil
}
