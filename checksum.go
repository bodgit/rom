package rom

import (
	"crypto/md5"
	"crypto/sha1"
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

func checksum(r io.Reader) ([][]byte, error) {
	c := crc32.NewIEEE()
	m := md5.New()
	s := sha1.New()

	if _, err := io.Copy(io.MultiWriter(c, m, s), r); err != nil {
		return nil, err
	}

	return [][]byte{c.Sum(nil)[:], m.Sum(nil)[:], s.Sum(nil)[:]}, nil
}

var extensionToChecksum = map[string]func(io.Reader) ([][]byte, error){
	".lnx": func(r io.Reader) ([][]byte, error) {
		var err error
		if r, _, err = lynxReader(r); err != nil {
			return nil, err
		}

		return checksum(r)
	},
	".nes": func(r io.Reader) ([][]byte, error) {
		var err error
		if r, _, err = nesReader(r); err != nil {
			return nil, err
		}

		return checksum(r)
	},
}

func needsDirectChecksum(filename string) bool {
	if _, ok := extensionToChecksum[filepath.Ext(filename)]; ok {
		return true
	}
	return false
}

func checksumFunction(filename string) func(io.Reader) ([][]byte, error) {
	if f, ok := extensionToChecksum[filepath.Ext(filename)]; ok {
		return f
	}
	return checksum
}
