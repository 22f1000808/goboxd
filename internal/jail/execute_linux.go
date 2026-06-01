//go:build linux
// +build linux

package jail

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const outputCap = 64 * 1024

func Execute(ctx context.Context, nsjailBin string, job Job) (Outcome, error) {
	if job.Workspace.CgroupPath != "" {
		if err := CreateCgroup(job.Workspace.CgroupPath, job.MemoryKB, job.MaxProcesses); err != nil {
			_ = err
		}
	}

	cfgPath := filepath.Join(job.Workspace.HostRoot, "nsjail.cfg")
	cfg, err := renderConfig(job)
	if err != nil {
		return Outcome{}, fmt.Errorf("render: %w", err)
	}

	if err := os.WriteFile(cfgPath, []byte(cfg), 0600); err != nil {
		return Outcome{}, fmt.Errorf("write cfg: %w", err)
	}

	cmd := exec.CommandContext(ctx, nsjailBin, "--config", cfgPath, "--really_quiet")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 2 * time.Second

	stdout := &capBuffer{cap: outputCap}
	stderr := &capBuffer{cap: outputCap}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if job.Stdin != "" {
		cmd.Stdin = bytes.NewReader([]byte(job.Stdin))
	}

	start := time.Now()
	err = cmd.Run()
	dur := time.Since(start)

	o := Outcome{
		Stdout:          stdout.Bytes(),
		Stderr:          stderr.Bytes(),
		StdoutTruncated: stdout.truncated,
		StderrTruncated: stderr.truncated,
		DurationMS:      dur.Milliseconds(),
	}

	if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
		o.TimedOut = ctx.Err() == context.DeadlineExceeded
	}

	if cmd.ProcessState != nil {
		o.ExitCode = cmd.ProcessState.ExitCode()
		if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			o.Signal = int(ws.Signal())
			// SIGKILL (9) indicates timeout from nsjail
			if o.Signal == 9 {
				o.TimedOut = true
			}
		}
	}

	// Also detect timeout if duration matches the expected timeout (within 100ms tolerance)
	if !o.TimedOut && job.WallTimeS > 0 {
		expectedDurMS := int64(job.WallTimeS * 1000)
		if o.DurationMS >= expectedDurMS-100 && o.DurationMS <= expectedDurMS+500 {
			o.TimedOut = true
		}
	}

	if peak, perr := readUint64File(filepath.Join(job.Workspace.CgroupPath, "memory.peak")); perr == nil {
		o.MemoryPeakKB = int64(peak / 1024)
	}

	cg := ReadCgroupOutcome(job.Workspace.CgroupPath)
	if cg.MemoryPeakKB > o.MemoryPeakKB {
		o.MemoryPeakKB = cg.MemoryPeakKB
	}
	o.OOMKilled = cg.OOMKilled
	o.PIDsExhausted = cg.PIDsExhausted

	if err != nil && cmd.ProcessState == nil {
		return o, fmt.Errorf("nsjail launch: %w", err)
	}

	return o, nil
}

type capBuffer struct {
	buf       bytes.Buffer
	cap       int
	truncated bool
}

func (c *capBuffer) Write(p []byte) (int, error) {
	if c.truncated {
		return len(p), nil
	}

	remaining := c.cap - c.buf.Len()
	if len(p) <= remaining {
		return c.buf.Write(p)
	}

	if remaining > 0 {
		c.buf.Write(p[:remaining])
	}

	c.buf.WriteString("\n[...output truncated...]")
	c.truncated = true
	return len(p), nil
}

func (c *capBuffer) Bytes() []byte {
	return c.buf.Bytes()
}
