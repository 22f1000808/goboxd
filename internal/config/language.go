package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	FilenameFixed          = "fixed"
	FilenameClientSupplied = "from_request"
)

type LanguageSpec struct {
	ID                     string     `yaml:"id"`
	Name                   string     `yaml:"name"`
	SourceFilename         string     `yaml:"source_filename,omitempty"`
	SourceFilenameStrategy string     `yaml:"source_filename_strategy,omitempty"`
	Artifact               string     `yaml:"artifact,omitempty"`
	Build                  *PhaseSpec `yaml:"build,omitempty"`
	Run                    *PhaseSpec `yaml:"run"`
	SmokeProbeCmd          string     `yaml:"smoke_probe_cmd,omitempty"`
	SmokeProbe             []string   `yaml:"smoke_probe,omitempty"`

	flagRules struct {
		build []FlagRule
		run   []FlagRule
	}
}

type PhaseSpec struct {
	Cmd           string   `yaml:"cmd"`
	Args          []string `yaml:"args,omitempty"`
	Limits        Limits   `yaml:"limits"`
	FlagAllowlist []string `yaml:"flag_allowlist,omitempty"`
}

type Limits struct {
	WallTimeS    int `yaml:"wall_time_s"`
	MemoryKB     int `yaml:"memory_kb"`
	MaxProcesses int `yaml:"max_processes"`
}

type Registry struct {
	languages map[string]*LanguageSpec
	order     []string
}

func (r *Registry) Lookup(id string) (*LanguageSpec, bool) {
	l, ok := r.languages[id]
	return l, ok
}

func (r *Registry) IDs() []string {
	return append([]string(nil), r.order...)
}

// All returns the language specs in the registry's stable order.
// Read-only; callers must not mutate the returned pointers.
func (r *Registry) All() []*LanguageSpec {
	out := make([]*LanguageSpec, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.languages[id])
	}
	return out
}

func (l *LanguageSpec) BuildFlagRules() []FlagRule {
	return l.flagRules.build
}

func (l *LanguageSpec) RunFlagRules() []FlagRule {
	return l.flagRules.run
}

type languagesFile struct {
	Languages []*LanguageSpec `yaml:"languages"`
}

func LoadLanguages(path string) (*Registry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	dec := yaml.NewDecoder(bytesReader(b))
	dec.KnownFields(true)
	var f languagesFile
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	reg := &Registry{languages: make(map[string]*LanguageSpec)}
	for _, ls := range f.Languages {
		if err := validateLanguage(ls); err != nil {
			return nil, fmt.Errorf("language %q: %w", ls.ID, err)
		}

		if _, dup := reg.languages[ls.ID]; dup {
			return nil, fmt.Errorf("duplicate language id %q", ls.ID)
		}

		ls.flagRules.run = ParseAllowlist(ls.Run.FlagAllowlist)
		if ls.Build != nil {
			ls.flagRules.build = ParseAllowlist(ls.Build.FlagAllowlist)
		}

		reg.languages[ls.ID] = ls
		reg.order = append(reg.order, ls.ID)
	}

	if len(reg.order) == 0 {
		return nil, fmt.Errorf("no languages defined")
	}

	sort.Strings(reg.order)
	return reg, nil
}

func validateLanguage(l *LanguageSpec) error {
	if l.ID == "" {
		return fmt.Errorf("missing id")
	}

	if l.Run == nil || l.Run.Cmd == "" {
		return fmt.Errorf("missing run.cmd")
	}

	if l.Run.Limits.WallTimeS <= 0 {
		return fmt.Errorf("missing run.limits.wall_time_s")
	}

	if l.Run.Limits.MemoryKB <= 0 {
		return fmt.Errorf("missing run.limits.memory_kb")
	}

	if l.Build != nil {
		if l.Build.Cmd == "" {
			return fmt.Errorf("missing build.cmd")
		}
		if l.Artifact == "" {
			return fmt.Errorf("language with build phase needs artifact")
		}
		if err := validateArgsTemplate(l.Build.Args); err != nil {
			return fmt.Errorf("build.args: %w", err)
		}
	}

	if err := validateArgsTemplate(l.Run.Args); err != nil {
		return fmt.Errorf("run.args: %w", err)
	}

	switch l.SourceFilenameStrategy {
	case "", FilenameFixed:
		l.SourceFilenameStrategy = FilenameFixed
		if l.SourceFilename == "" {
			return fmt.Errorf("fixed strategy needs source_filename")
		}
	case FilenameClientSupplied:
	default:
		return fmt.Errorf("unknown source_filename_strategy %q", l.SourceFilenameStrategy)
	}

	return nil
}

func validateArgsTemplate(args []string) error {
	for _, a := range args {
		if a == "{{flags}}" {
			continue
		}
		if strings.Contains(a, "{{flags}}") {
			return fmt.Errorf("element %q mixes {{flags}} with other text;\n{{flags}} must be a whole element", a)
		}
	}
	return nil
}
