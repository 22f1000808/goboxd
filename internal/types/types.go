package types

type RunRequest struct {
	Language       string `json:"language"`
	SourceFilename string `json:"source_filename,omitempty"`
	Source         string `json:"source"`
	Build          *Phase `json:"build,omitempty"`
	Run            *Phase `json:"run,omitempty"`
	Tests          []Test `json:"tests"`
}

type Phase struct {
	Limits *Limits  `json:"limits,omitempty"`
	Flags  []string `json:"flags,omitempty"`
}

type Limits struct {
	WallTimeS    int `json:"wall_time_s,omitempty"`
	MemoryKB     int `json:"memory_kb,omitempty"`
	MaxProcesses int `json:"max_processes,omitempty"`
}

type Test struct {
	Stdin          string `json:"stdin"`
	ExpectedStdout string `json:"expected_stdout"`
}

type RunResponse struct {
	Status string       `json:"status"`
	Build  *BuildResult `json:"build,omitempty"`
	Tests  []TestResult `json:"tests"`
}

type BuildResult struct {
	Status     string `json:"status"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}

type TestResult struct {
	Status       string `json:"status"`
	Stdout       string `json:"stdout,omitempty"`
	Stderr       string `json:"stderr,omitempty"`
	DurationMS   int64  `json:"duration_ms,omitempty"`
	MemoryPeakKB int64  `json:"memory_peak_kb,omitempty"`
}

const (
	StatusAccepted             = "accepted"
	StatusWrongOutput          = "wrong_output"
	StatusOutputWhitespaceDiff = "output_whitespace_diff"
	StatusTimeExceeded         = "time_exceeded"
	StatusMemoryExceeded       = "memory_exceeded"
	StatusRuntimeError         = "runtime_error"
	StatusBuildFailed          = "build_failed"
	StatusInternalError        = "internal_error"
	StatusNotExecuted          = "not_executed"
)

const (
	BuildStatusOK            = "ok"
	BuildStatusFailed        = "failed"
	BuildStatusInternalError = "internal_error"
)
