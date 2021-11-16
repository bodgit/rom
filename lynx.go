package rom

import (
	"bytes"
	"io"
)

const (
	lynxExtension  = ".lnx"
	lynxHeaderSize = 64
)

// See the following for reference:
//
// * https://atarigamer.com/lynx/lnx2lyx

func lynxReader(r io.Reader) (io.Reader, uint64, error) {
	b := new(bytes.Buffer)
	if _, err := io.CopyN(b, r, lynxHeaderSize); err != nil {
		return nil, 0, err
	}

	if bytes.Compare(b.Bytes()[0:4], []byte{'L', 'Y', 'N', 'X'}) != 0 {
		return io.MultiReader(b, r), 0, nil
	}

	return r, lynxHeaderSize, nil
}
