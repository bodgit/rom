package synchronizer

import (
	"fmt"

	"github.com/bodgit/rom"
)

func checksumToString(c []byte) string {
	return fmt.Sprintf("%x", c)
}

type checksum struct {
	Type  rom.Checksum
	Value string
	Size  uint64
}
