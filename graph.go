package main

import (
	"strings"

	"github.com/emicklei/dot"
)

func RenderWorkspacesDetailed(workspaces map[string]*Workspace) *dot.Graph {
	g := dot.NewGraph()

	// draw workspaces
	for name, workspace := range workspaces {
		// draw workspace
		workspace.graphElement = g.Subgraph(name, dot.ClusterOption{})

		// draw outputs
		if len(workspace.Outputs) > 0 {
			outputs := workspace.graphElement.Subgraph("outputs", dot.ClusterOption{})
			for i, output := range workspace.Outputs {
				workspace.Outputs[i].graphElement = outputs.Node(output.Name)
			}
		}

		// draw inputs
		if len(workspace.Inputs) > 0 {
			inputs := workspace.graphElement.Subgraph("inputs", dot.ClusterOption{})
			for i, input := range workspace.Inputs {
				workspace.Inputs[i].graphElement = inputs.Node(input.Name)
			}
		}
	}

	// draw relations/dependencies
	for _, workspace := range workspaces {
		for i, input := range workspace.Inputs {
			if input.ReferesTo != nil {
				g.Edge(input.graphElement, input.ReferesTo.graphElement).Attr("label", strings.Join(input.InFile, ", "))
			} else {
				workspace.Inputs[i].graphElement.Attr("color", "red")
			}
		}
	}

	return g
}

func RenderWorkspaces(workspaces map[string]*Workspace) *dot.Graph {
	g := dot.NewGraph(dot.Directed)

	nodes := map[string]dot.Node{}

	// draw workspaces
	for name, _ := range workspaces {
		nodes[name] = g.Node(name)
	}

	// draw relations/dependencies
	for name, workspace := range workspaces {
		for _, dep := range workspace.Dependencies {
			for otherName, other := range workspaces {
				if dep.equals(other.RemoteState) {
					g.Edge(nodes[name], nodes[otherName])
				}
			}
		}
	}

	return g
}
