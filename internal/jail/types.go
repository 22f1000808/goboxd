package jail

type Workspace struct {
	HostRoot    string
	SandboxPath string
	CgroupPath  string
}

type Mount struct {
	HostPath string
	JailPath string
	ReadOnly bool
}

type Job struct {
	Name         string
	Workspace    Workspace
	Cmd          string
	Args         []string
	Stdin        string
	Mounts       []Mount
	WallTimeS    int
	MemoryKB     int
	MaxProcesses int
	FsizeMB      int
	AllowNetwork bool
}

type Outcome struct {
	ExitCode        int
	Signal          int
	TimedOut        bool
	Stdout          []byte
	Stderr          []byte
	StdoutTruncated bool
	StderrTruncated bool
	DurationMS      int64
	MemoryPeakKB    int64
	OOMKilled       bool
	PIDsExhausted   bool
}
