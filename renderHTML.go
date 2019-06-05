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
  <style type="text/css">
    .manual {
      padding-left: 40px;
	  border-left: solid gray 3px;
    }
    pre {
      padding: 5px;
	  background-color: #777;
	  color: white;
      overflow-x: auto;
      white-space: pre-wrap;
      white-space: -moz-pre-wrap;
      white-space: -pre-wrap;
      white-space: -o-pre-wrap;
      word-wrap: break-word;
	}
	th, td {
	  border-bottom: 1px solid #ddd;
	  padding: 15px;
      text-align: left;
    }
    .container {
	  max-width: 800px;
	  width: 90%;
	  margin: 0 auto;
    }
  </style>
</head>
<body>
  <div class="container">
  {{range $tier, $workspaces := .}}
  <h1>({{$tier}}) Tier {{ $tier }}</h1>
  {{range $i, $workspace := $workspaces}}
  <h2>({{$tier}}.{{$i}}) Workspace <code>{{$workspace.Root}}</code></h2>
  {{if $workspace.PreManualRendered}}
  <h3>Manual Pre-Work</h3>
  <div class="manual">
  {{$workspace.PreManualRendered}}
  </div>
  {{end}}
  <h3>Terraform</h3>
  <pre><code>
  (cd {{ $workspace.Root }} && terraform apply)
  </code></pre>
  {{if $workspace.PostManualRendered}}
  <h3>Manual Post-Work</h3>
  <div class="manual">
  {{$workspace.PostManualRendered}}
  </div>
  {{end}}{{end}}{{end}}
  </div>
</body>
</html>`
	var out bytes.Buffer
	tmpl := template.Must(template.New("test").Parse(tmplSrc))
	tmpl.Execute(&out, plan)

	return out.String()
}
