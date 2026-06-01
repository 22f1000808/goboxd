package types

type RunRequest struct {
	Language         string `json:"language"`
	Source           string `json:"source"`
	SourceFilename   string `json:"source_filename,omitempty"`
	ArtifactFilename string `json:"artifact_filename,omitempty"`
	Build            *Phase `json:"build,omitempty"`
	Run              *Phase `json:"run,omitempty"`
	Tests            []Test `json:"tests"`
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
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMS int64  `json:"duration_ms"`
}

type TestResult struct {
	Status       string `json:"status"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	DurationMS   int64  `json:"duration_ms"`
	MemoryPeakKB int64  `json:"memory_peak_kb"`
}

const (
	StatusAccepted                  = "accepted"
	StatusWrongOutput               = "wrong_output"
	StatusOutputWhitespaceMismatch  = "output_whitespace_mismatch"
	StatusTimeExceeded              = "time_exceeded"
	StatusMemoryExceeded            = "memory_exceeded"
	StatusRuntimeError              = "runtime_error"
	StatusBuildFailed               = "build_failed"
	StatusInternalError             = "internal_error"
	StatusNotExecuted               = "not_executed"
)

const (
	BuildStatusOK            = "ok"
	BuildStatusFailed        = "failed"
	BuildStatusInternalError = "internal_error"
)
