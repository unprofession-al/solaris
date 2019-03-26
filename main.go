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
	flag.StringVar(&format, "f", "dot", "output format ('dot' or 'json')")
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

	graph := RenderWorkspaces(workspaces)

	if format == "dot" {
		fmt.Println(graph.String())
		fmt.Printf("\n/*\n  Use 'solaris ... | fdp -Tsvg > out.svg' or\n  similar to generate a vector visualization\n*/\n")
	} else {
		printJSON(workspaces)
	}

}
