package jail

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type CgroupOutcome struct {
	MemoryPeakKB  int64
	OOMKilled     bool
	PIDsExhausted bool
}

func CreateCgroup(path string, memKB, pidsMax int) error {
	if err := os.Mkdir(path, 0755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("mkdir cgroup: %w", err)
	}

	if memKB > 0 {
		if err := os.WriteFile(filepath.Join(path, "memory.max"),
			[]byte(strconv.FormatInt(int64(memKB)*1024, 10)), 0644); err != nil {
			return fmt.Errorf("write memory.max: %w", err)
		}
	}

	if pidsMax > 0 {
		if err := os.WriteFile(filepath.Join(path, "pids.max"),
			[]byte(strconv.Itoa(pidsMax)), 0644); err != nil {
			return fmt.Errorf("write pids.max: %w", err)
		}
	}

	return nil
}

func ReadCgroupOutcome(path string) CgroupOutcome {
	var o CgroupOutcome
	if path == "" {
		return o
	}

	if peak, err := readUint64File(filepath.Join(path, "memory.peak")); err == nil {
		o.MemoryPeakKB = int64(peak / 1024)
	}

	if ev, err := readEventsFile(filepath.Join(path, "memory.events")); err == nil {
		if v, ok := ev["oom_kill"]; ok && v > 0 {
			o.OOMKilled = true
		}
	}

	if ev, err := readEventsFile(filepath.Join(path, "pids.events")); err == nil {
		if v, ok := ev["max"]; ok && v > 0 {
			o.PIDsExhausted = true
		}
	}

	return o
}

func RemoveCgroup(path string) error {
	if path == "" {
		return nil
	}

	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func readEventsFile(p string) (map[string]uint64, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	out := map[string]uint64{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		out[fields[0]] = v
	}
	return out, nil
}
