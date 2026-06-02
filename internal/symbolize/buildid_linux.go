//go:build linux

package symbolize

import (
	"debug/elf"
	"encoding/hex"
	"fmt"
)

func readELFBuildID(path string) string {
	f, err := elf.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sec := f.Section(".note.gnu.build-id")
	if sec == nil {
		return ""
	}
	data, err := sec.Data()
	if err != nil || len(data) < 16 {
		return ""
	}
	// ELF note: namesz, descsz, type, name, desc
	off := 0
	for off+12 <= len(data) {
		namesz := int(data[off])
		descsz := int(data[off+4])
		_ = data[off+8] // type
		off += 12
		nameEnd := off + ((namesz + 3) &^ 3)
		descEnd := nameEnd + ((descsz + 3) &^ 3)
		if descEnd > len(data) {
			break
		}
		desc := data[nameEnd : nameEnd+descsz]
		if len(desc) > 0 {
			return hex.EncodeToString(desc)
		}
		off = descEnd
	}
	return ""
}

// BuildIDKey returns a cache key for a module.
func BuildIDKey(path, buildID string) string {
	if buildID != "" {
		return buildID
	}
	return fmt.Sprintf("path:%s", path)
}
