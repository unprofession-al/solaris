package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
)

var root string
var format string

func init() {
	flag.StringVar(&root, "r", ".", "root path")
	flag.StringVar(&format, "f", "dot", "output format ('dot', 'dotdetailed', 'json' or 'lint')")
}

func printJSON(in interface{}) {
	json, err := json.MarshalIndent(in, "", "    ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(json))
}

func main() {
	flag.Parse()

	workspaces, err := GetWorkspaces(root, []string{"templates"})
	if err != nil {
		panic(err)
	}

	switch format {
	case "dot":
		graph := RenderWorkspaces(workspaces)
		fmt.Println(graph.String())
		fmt.Printf("\n/*\n  Use 'solaris ... | fdp -Tsvg > out.svg' or\n  similar to generate a vector visualization\n*/\n")
	case "dotdetailed":
		graph := RenderWorkspacesDetailed(workspaces)
		fmt.Println(graph.String())
		fmt.Printf("\n/*\n  Use 'solaris ... | fdp -Tsvg > out.svg' or\n  similar to generate a vector visualization\n*/\n")
	case "json":
		printJSON(workspaces)
	case "exec":
		BuildExecutionPlan(workspaces, []string{})
	case "lint":
		errs := Lint(workspaces)
		for k, v := range errs {
			fmt.Println(k)
			for _, e := range v {
				fmt.Printf("\t%s\n", e)
			}
		}
	}
}
