package main

import (
	"bytes"
	"text/template"
)

func RenderExecutionPlanAsHTML(plan [][]*Workspace) string {
	tmplSrc := `<!doctype html>

<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Execution Plan</title>
  {{range $tier, $workspaces := .}}
  <h1>Tier {{ $tier }}</h1>
  {{range $i, $workspace := $workspaces}}
  <h2>Workspace <code>{{$workspace.Root}}</code></h2>
  {{if $workspace.PreManualRendered}}
  <h3>Manual Pre-Work</h3>
  {{$workspace.PreManualRendered}}
  {{end}}
  <h3>Terraform</h3>
  <code>
  (cd {{ $workspace.Root }} && terraform apply)
  </code>
  {{if $workspace.PostManualRendered}}
  <h3>Manual Post-Work</h3>
  {{$workspace.PostManualRendered}}
  {{end}}
  {{end}}
  {{end}}

</head>

</html>`
	var out bytes.Buffer
	tmpl := template.Must(template.New("test").Parse(tmplSrc))
	tmpl.Execute(&out, plan)

	return out.String()
}
