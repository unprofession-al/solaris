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
	flag.StringVar(&format, "f", "dot", "output format ('dot' or 'text')")
}

func printJSON(in interface{}) {
	json, err := json.MarshalIndent(in, "", "    ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(json))
	fmt.Println("---")
}

type Terraform struct {
	State        RemoteState
	Dependencies []RemoteState
}

func main() {
	flag.Parse()

	/*
		dirs, err := getWorkspaceList(root)
		if err != nil {
			panic(err)
		}

		tfWorkspaces := map[string]Terraform{}

		for dir, files := range dirs {
			state := RemoteState{}
			dependencies := []RemoteState{}

			for _, file := range files {
				c, err := readFile(dir, file)
				if err != nil {
					fmt.Printf("Error while reading file %s/%s: %s", dir, file, err.Error())
					os.Exit(1)
				}

				rs, err := getRemoteState(file, c)
				if err != nil {
					panic(err)
				}
				if rs.Key != "" {
					state = rs
				}

				deps, err := getDependencies(file, c)
				if err != nil {
					panic(err)
				}
				if len(deps) > 0 {
					dependencies = append(dependencies, deps...)
				}
			}

			tf := Terraform{
				Dependencies: dependencies,
				State:        state,
			}
			tfWorkspaces[strings.TrimPrefix(dir, root)] = tf
		}

		g := dot.NewGraph(dot.Directed)
		nodes := map[string]dot.Node{}

		for name, _ := range tfWorkspaces {
			nodes[name] = g.Node(name)
		}

		for name, tf := range tfWorkspaces {
			if len(tf.Dependencies) > 0 {
				if format == "text" {
					fmt.Println(name)
				}
				for _, dep := range tf.Dependencies {
					for otherName, otherTf := range tfWorkspaces {
						if dep.equals(otherTf.State) {
							g.Edge(nodes[name], nodes[otherName])
							if format == "text" {
								fmt.Println("   " + otherName)
							}
						}
					}
				}
			}
		}
		if format == "dot" {
			fmt.Println(g.String())
		}
	*/

	workspaces, err := GetWorkspaces(root, []string{"templates"})
	if err != nil {
		panic(err)
	}
	workspaces = workspaces
	/*
		for name, workspace := range workspaces {
			fmt.Printf("%s\n", name)
			for _, dep := range workspace.Dependencies {
				fmt.Printf("   %s\n", dep.Name)
			}
		}
	*/
}
