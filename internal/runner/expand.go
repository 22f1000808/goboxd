package runner

import "strings"

const (
	tplFlags    = "{{flags}}"
	tplSource   = "{{source}}"
	tplArtifact = "{{artifact}}"
)

func expandArgs(tmpl []string, source, artifact string, flags []string) []string {
	out := make([]string, 0, len(tmpl)+len(flags))
	for _, a := range tmpl {
		if a == tplFlags {
			out = append(out, flags...)
			continue
		}
		a = strings.ReplaceAll(a, tplSource, source)
		a = strings.ReplaceAll(a, tplArtifact, artifact)
		out = append(out, a)
	}
	return out
}

func expandCmd(cmd, source, artifact string) string {
	cmd = strings.ReplaceAll(cmd, tplSource, source)
	cmd = strings.ReplaceAll(cmd, tplArtifact, artifact)
	return cmd
}
