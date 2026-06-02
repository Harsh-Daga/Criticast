//go:build linux

package symbolize

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ModulesFromPID reads /proc/<pid>/maps and returns executable mappings for targetExe.
// targetExe may be "" to include all executable file-backed mappings.
func ModulesFromPID(pid int, targetExe string) ([]Module, error) {
	path := fmt.Sprintf("/proc/%d/maps", pid)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("symbolize: open %s: %w", path, err)
	}
	defer f.Close()

	absTarget := ""
	if targetExe != "" {
		absTarget, err = filepath.Abs(targetExe)
		if err != nil {
			absTarget = targetExe
		}
	}

	var mods []Module
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		m, ok := parseMapsLine(sc.Text())
		if !ok {
			continue
		}
		if absTarget != "" && !sameExecutable(m.Path, absTarget) {
			continue
		}
		if m.BuildID == "" {
			m.BuildID = readELFBuildID(m.Path)
		}
		mods = append(mods, m)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return mods, nil
}

func parseMapsLine(line string) (Module, bool) {
	// start-end perms offset dev inode pathname
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return Module{}, false
	}
	if !strings.Contains(fields[1], "x") {
		return Module{}, false
	}
	parts := strings.Split(fields[0], "-")
	if len(parts) != 2 {
		return Module{}, false
	}
	start, err1 := strconv.ParseUint(parts[0], 16, 64)
	end, err2 := strconv.ParseUint(parts[1], 16, 64)
	if err1 != nil || err2 != nil {
		return Module{}, false
	}
	path := ""
	if len(fields) >= 6 {
		path = fields[5]
	}
	if path == "" || strings.HasPrefix(path, "[") {
		return Module{}, false
	}
	return Module{Path: path, Start: start, End: end}, true
}

func sameExecutable(a, b string) bool {
	if a == b {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA == nil && errB == nil && ra == rb {
		return true
	}
	return filepath.Base(a) == filepath.Base(b)
}
