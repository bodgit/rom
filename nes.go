package rom

import (
	"bytes"
	"io"
)

const (
	nesExtension  = ".nes"
	nesHeaderSize = 16
)

// See the following for reference:
//
// * https://wiki.nesdev.com/w/index.php/INES
// * https://wiki.nesdev.com/w/index.php/NES_2.0

func nesReader(r io.Reader) (io.Reader, uint64, error) {
	b := new(bytes.Buffer)
	if _, err := io.CopyN(b, r, nesHeaderSize); err != nil {
		return nil, 0, err
	}

	if bytes.Compare(b.Bytes()[0:4], []byte{'N', 'E', 'S', 0x1a}) != 0 {
		return io.MultiReader(b, r), 0, nil
	}

	return r, nesHeaderSize, nil
}
