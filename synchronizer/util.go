package synchronizer

import (
	"github.com/bodgit/rom"
	"github.com/bodgit/rom/dat"
)

func gameFilename(game dat.Game) string {
	return game.Name + ".zip"
}

func romChecksum(r dat.ROM, c rom.Checksum) checksum {
	return checksum{
		Type:  c,
		Value: r.Checksum(c),
		Size:  r.Size,
	}
}
