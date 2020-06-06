package rom

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"errors"
	"hash"
	"hash/crc32"
	"io"
	"path/filepath"
)

// Checksum is used to specify a checksum/hash type
type Checksum int

// Supported checksum/hash types
const (
	CRC32 Checksum = iota
	MD5
	SHA1
)

var errUnknownChecksum = errors.New("unknown checksum")

func checksum(r io.Reader, c Checksum) ([]byte, error) {
	var h hash.Hash

	switch c {
	case CRC32:
		h = crc32.NewIEEE()
	case MD5:
		h = md5.New()
	case SHA1:
		h = sha1.New()
	default:
		return nil, errUnknownChecksum
	}

	if _, err := io.Copy(h, r); err != nil {
		return nil, err
	}

	return h.Sum(nil)[:], nil
}

var extensionToChecksum = map[string]func(io.Reader, Checksum) ([]byte, error){
	".nes": func(r io.Reader, c Checksum) ([]byte, error) {
		b := new(bytes.Buffer)
		if _, err := io.CopyN(b, r, 16); err != nil {
			return nil, err
		}

		if bytes.Compare(b.Bytes()[0:4], []byte{'N', 'E', 'S', 0x1a}) != 0 {
			r = io.MultiReader(b, r)
		}

		return checksum(r, c)
	},
}

func needsDirectChecksum(filename string) bool {
	if _, ok := extensionToChecksum[filepath.Ext(filename)]; ok {
		return true
	}
	return false
}

func checksumFunction(filename string) func(io.Reader, Checksum) ([]byte, error) {
	if f, ok := extensionToChecksum[filepath.Ext(filename)]; ok {
		return f
	}
	return checksum
}
