package main

import (
	"text/template"
	"fmt"
)

const nsjailTmpl = `name: "goboxd"
{{ if not .AllowNetwork -}}
clone_newnet: true
{{- end }}
`

func main() {
	_, err := template.New("nsjail").Parse(nsjailTmpl)
	fmt.Println("Error:", err)
}
