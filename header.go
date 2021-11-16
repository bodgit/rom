package rom

import (
	"io"
	"path/filepath"
)

func headerSize(_ io.Reader) (uint64, error) {
	return 0, nil
}

var extensionToHeaderSize = map[string]func(io.Reader) (uint64, error){
	lynxExtension: func(r io.Reader) (uint64, error) {
		_, hs, err := lynxReader(r)
		if err != nil {
			return 0, err
		}

		return hs, nil
	},
	nesExtension: func(r io.Reader) (uint64, error) {
		_, hs, err := nesReader(r)
		if err != nil {
			return 0, err
		}

		return hs, nil
	},
}

func hasHeader(filename string) bool {
	if _, ok := extensionToHeaderSize[filepath.Ext(filename)]; ok {
		return true
	}
	return false
}

func headerSizeFunction(filename string) func(io.Reader) (uint64, error) {
	if f, ok := extensionToHeaderSize[filepath.Ext(filename)]; ok {
		return f
	}
	return headerSize
}
