// internal/config/server.go
package config

import (
	"fmt"
	"os"
	"runtime"

	"gopkg.in/yaml.v3"
)

// ServerConfig is loaded from configs/server.yaml. Env vars prefixed with
// GOBOXD_ override individual fields (handled in cmd/goboxd, not here).
type ServerConfig struct {
	HTTPAddr               string `yaml:"http_addr"`
	MaxConcurrentJobs      int    `yaml:"max_concurrent_jobs"`
	MaxQueueDepth          int    `yaml:"max_queue_depth"`
	DrainTimeoutS          int    `yaml:"drain_timeout_s"`
	ReadyzCacheTTLS        int    `yaml:"readyz_cache_ttl_s"`
	MaxSourceBytes         int    `yaml:"max_source_bytes"`
	MaxStdinBytes          int    `yaml:"max_stdin_bytes"`
	MaxExpectedStdoutBytes int    `yaml:"max_expected_stdout_bytes"`
	MaxTests               int    `yaml:"max_tests"`
	JailRootDir            string `yaml:"jail_root_dir"`
	NSJailBinary           string `yaml:"nsjail_binary"`
	CgroupV2Mount          string `yaml:"cgroupv2_mount"`
}

func LoadServer(path string) (*ServerConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	dec := yaml.NewDecoder(bytesReader(b))
	dec.KnownFields(true)
	var c ServerConfig
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	c.applyDefaults()
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("validate %s: %w", path, err)
	}
	return &c, nil
}

func (c *ServerConfig) applyDefaults() {
	if c.HTTPAddr == "" {
		c.HTTPAddr = ":8080"
	}
	if c.MaxConcurrentJobs == 0 {
		c.MaxConcurrentJobs = runtime.NumCPU()
	}
	// MaxQueueDepth defaults to 0 = unbounded per spec §07 ("requests
	// queue rather than fail"). Operators may set a positive value as a
	// safety valve against pathological bursts.
	if c.DrainTimeoutS == 0 {
		c.DrainTimeoutS = 45
	}
	if c.ReadyzCacheTTLS == 0 {
		c.ReadyzCacheTTLS = 30
	}
	if c.MaxSourceBytes == 0 {
		c.MaxSourceBytes = 256 * 1024
	}
	if c.MaxStdinBytes == 0 {
		c.MaxStdinBytes = 64 * 1024
	}
	if c.MaxExpectedStdoutBytes == 0 {
		c.MaxExpectedStdoutBytes = 1024 * 1024 // 1 MiB
	}
	if c.MaxTests == 0 {
		c.MaxTests = 50
	}
	if c.JailRootDir == "" {
		c.JailRootDir = "/var/lib/goboxd/jails"
	}
	if c.NSJailBinary == "" {
		c.NSJailBinary = "/usr/local/bin/nsjail"
	}
	if c.CgroupV2Mount == "" {
		c.CgroupV2Mount = "/sys/fs/cgroup"
	}
}

func (c *ServerConfig) validate() error {
	if c.MaxConcurrentJobs < 1 {
		return fmt.Errorf("max_concurrent_jobs must be >= 1")
	}
	if c.MaxQueueDepth < 0 {
		return fmt.Errorf("max_queue_depth must be >= 0")
	}
	if c.MaxSourceBytes < 1 {
		return fmt.Errorf("max_source_bytes must be >= 1")
	}
	if c.MaxStdinBytes < 0 {
		return fmt.Errorf("max_stdin_bytes must be >= 0")
	}
	if c.MaxTests < 1 {
		return fmt.Errorf("max_tests must be >= 1")
	}
	return nil
}
