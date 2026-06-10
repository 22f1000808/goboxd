package jail

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

func pbEscape(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '"':
			b.WriteString("\\\"")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

const nsjailTmpl = `name: "{{ pbEscape .Name }}"
mode: ONCE
hostname: "goboxd"
cwd: "/work"

time_limit: {{ .WallTimeS }}
rlimit_as: {{ .RlimitAsKB }}
rlimit_fsize: {{ .FsizeMB }}
rlimit_nofile: 64
rlimit_nproc: 0
rlimit_stack: 0

{{ if .CgroupPath -}}
cgroupv2_mount: "{{ pbEscape .CgroupPath }}"
{{- end }}

envar: "PATH=/bin:/usr/bin:/usr/local/bin"
envar: "HOME=/tmp"
envar: "GOPATH=/tmp/gopath"
envar: "GOCACHE=/tmp/gocache"
envar: "GOROOT=/usr/lib/go-1.19"
envar: "JAVA_TOOL_OPTIONS=-Dfile.encoding=UTF-8"

{{ if not .AllowNetwork -}}
clone_newnet: true
{{- end }}

clone_newuser: true
clone_newns: true
clone_newpid: true
clone_newipc: true
clone_newuts: true
clone_newcgroup: true

uidmap { inside_id: "0" outside_id: "0" count: 1 }
gidmap { inside_id: "0" outside_id: "0" count: 1 }

mount { src: "/usr" dst: "/usr" is_bind: true rw: false }
mount { src: "/lib" dst: "/lib" is_bind: true rw: false }
mount { src: "/lib64" dst: "/lib64" is_bind: true rw: false }
mount { src: "/bin" dst: "/bin" is_bind: true rw: false }
mount { src: "/sbin" dst: "/sbin" is_bind: true rw: false }
mount { src: "/etc" dst: "/etc" is_bind: true rw: false }
{{ range .Mounts -}}
mount { src: "{{ pbEscape .HostPath }}" dst: "{{ pbEscape .JailPath }}" is_bind: true rw: {{ if .ReadOnly }}false{{ else }}true{{ end }} }
{{ end -}}
mount { dst: "/tmp" fstype: "tmpfs" rw: true is_bind: false }
mount { dst: "/dev" fstype: "tmpfs" rw: false is_bind: false }
mount { dst: "/proc" fstype: "proc" rw: false is_bind: false }

exec_bin {
  path: "{{ pbEscape .Cmd }}"
  {{ range .Args -}}
  arg: "{{ pbEscape . }}"
  {{ end -}}
}
`

type tmplData struct {
	Name         string
	WallTimeS    int
	RlimitAsKB   int
	FsizeMB      int
	CgroupPath   string
	AllowNetwork bool
	Mounts       []Mount
	Cmd          string
	Args         []string
}

func renderConfig(j Job) (string, error) {
	t, err := template.New("nsjail").Funcs(template.FuncMap{
		"pbEscape": pbEscape,
	}).Parse(nsjailTmpl)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	fsize := j.FsizeMB
	if fsize == 0 {
		fsize = 16
	}

	d := tmplData{
		Name:         j.Name,
		WallTimeS:    j.WallTimeS,
		RlimitAsKB:   4 * j.MemoryKB,
		FsizeMB:      fsize,
		CgroupPath:   j.Workspace.CgroupPath,
		AllowNetwork: j.AllowNetwork,
		Mounts:       j.Mounts,
		Cmd:          j.Cmd,
		Args:         j.Args,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}

	return buf.String(), nil
}
