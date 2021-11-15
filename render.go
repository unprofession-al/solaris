package main

import (
	"bytes"
	"text/template"
)

func GetTemplateFuncMap() template.FuncMap {
	funcMap := template.FuncMap{
		"mod": func(i, j int) bool { return i%j == 0 },
	}
	return funcMap
}

func RenderExecutionPlan(plan [][]*Workspace, tmplSrc string) string {
	var out bytes.Buffer
	tmpl := template.Must(template.New("test").Funcs(GetTemplateFuncMap()).Parse(tmplSrc))
	tmpl.Execute(&out, plan)

	return out.String()
}
