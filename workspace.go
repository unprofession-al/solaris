package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/emicklei/dot"
	"github.com/hashicorp/hcl"
	jmespath "github.com/jmespath/go-jmespath"
)

const tfext = ".tf"

// GetWorkspaces generates
func GetWorkspaces(root string, ignore []string) (map[string]*Workspace, error) {
	workspaces := map[string]*Workspace{}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		for _, i := range ignore {
			if strings.Contains(path, i) {
				return nil
			}
		}

		if filepath.Ext(path) == tfext {
			workspacePath, filename := filepath.Split(path)
			if w, found := workspaces[workspacePath]; found {
				w.Files[filename] = &File{}
			} else {
				files := map[string]*File{filename: &File{}}
				workspaces[workspacePath] = &Workspace{
					Files: files,
					Root:  workspacePath,
				}
			}
		}
		return nil
	})
	if err != nil {
		return workspaces, err
	}

	// fetch info per workspace
	for path, workspace := range workspaces {
		err = workspace.readFiles(path)
		if err != nil {
			return workspaces, err
		}

		err = workspace.getRemoteState()
		if err != nil {
			return workspaces, err
		}

		err = workspace.getDependencies()
		if err != nil {
			return workspaces, err
		}

		err = workspace.getInputs()
		if err != nil {
			return workspaces, err
		}

		err = workspace.getOutputs()
		if err != nil {
			return workspaces, err
		}
	}

	// get relations between workspace inputs and outputs
	for _, workspace := range workspaces {
		for i, input := range workspace.Inputs {
			for _, dep := range workspaces {
				if input.Dependency != nil && input.Dependency.equals(dep.RemoteState) {
					for o, output := range dep.Outputs {
						if input.Name == output.Name {
							workspace.Inputs[i].ReferesTo = &dep.Outputs[o]
							dep.Outputs[o].ReferedBy = append(dep.Outputs[o].ReferedBy, &workspace.Inputs[i])
						}

					}
				}
			}
		}
	}

	return workspaces, err
}

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

func Lint(workspaces map[string]*Workspace) map[string][]string {
	out := map[string][]string{}

	// check for unused outputs
	outputErrors := []string{}
	for name, workspace := range workspaces {
		for _, output := range workspace.Outputs {
			if len(output.ReferedBy) == 0 {
				outputErrors = append(outputErrors, fmt.Sprintf("output '%s' of workspace '%s' (in file '%s') seems to be unused", output.Name, name, output.InFile))
			}
		}
	}
	if len(outputErrors) > 0 {
		out["Usused Outputs"] = outputErrors
	}

	// check for inexistent inputs
	inputErrors := []string{}
	for name, workspace := range workspaces {
		for _, input := range workspace.Inputs {
			if input.ReferesTo == nil {
				inputErrors = append(inputErrors, fmt.Sprintf("input '%s' of workspace '%s' (in file '%s') seems refer to an inexistent output", input.FullName, name, input.InFile))
			}
		}
	}
	if len(inputErrors) > 0 {
		out["Inexistent Inputs"] = inputErrors
	}

	// check for unused terraform_remote_state data sources
	dataSourceErrors := []string{}
	for name, workspace := range workspaces {
		for _, dep := range workspace.Dependencies {
			depUsed := false
			for _, input := range workspace.Inputs {
				if input.Dependency != nil && input.Dependency.equals(dep) {
					depUsed = true
				}
			}
			if !depUsed {
				dataSourceErrors = append(dataSourceErrors, fmt.Sprintf("terraform_remote_state data source '%s' in workspace '%s' (in file '%s') seems to be unused", dep.Name, name, dep.InFile))
			}
		}
	}
	if len(dataSourceErrors) > 0 {
		out["Unused terraform_remote_state data sources"] = dataSourceErrors
	}

	// check for circular dependencies
	var checkCircular func(ws *Workspace, wsname string, dejavu []string) error
	checkCircular = func(ws *Workspace, wsname string, dejavu []string) error {
		for _, v := range dejavu {
			if wsname == v {
				return fmt.Errorf("%s -> %s", strings.Join(dejavu, " -> "), wsname)
			}
		}

		dejavu = append(dejavu, wsname)

		for _, input := range ws.Inputs {
			if input.ReferesTo == nil {
				continue
			}

			err := checkCircular(input.ReferesTo.BelongsTo, input.ReferesTo.BelongsTo.Root, dejavu)
			if err != nil {
				return err
			}
		}
		return nil
	}
	circularErrors := []string{}
	for name, workspace := range workspaces {
		err := checkCircular(workspace, name, []string{})

		if err != nil {
			circularErrors = append(circularErrors, fmt.Sprintf("circular dependency in workspace '%s': '%s'", name, err))
		}
	}
	if len(circularErrors) > 0 {
		out["Circular Dependencies"] = circularErrors
	}

	return out
}

type Workspace struct {
	Files        map[string]*File `json:"-"`
	Root         string           `json:"root"`
	RemoteState  RemoteState      `json:"remote_state"`
	Dependencies []RemoteState    `json:"dependencies"`
	Inputs       []Input          `json:"inputs"`
	Outputs      []Output         `json:"outputs"`
	graphElement *dot.Graph
}

type File struct {
	Raw          []byte
	Unmarshalled interface{}
}

type Input struct {
	Name         string       `json:"name"`
	FullName     string       `json:"full_name"`
	Dependency   *RemoteState `json:"dependency"`
	ReferesTo    *Output      `json:"referes_to"`
	InFile       []string     `json:"in_file"`
	BelongsTo    *Workspace   `json:"-"`
	graphElement dot.Node
}

type Output struct {
	Name         string      `json:"name"`
	Value        interface{} `json:"-"`
	InFile       string      `json:"in_file"`
	ReferedBy    []*Input    `json:"-"`
	BelongsTo    *Workspace  `json:"-"`
	graphElement dot.Node
}

func (ws *Workspace) readFiles(basepath string) error {
	for filename, file := range ws.Files {
		raw, err := ioutil.ReadFile(basepath + filename)
		if err != nil {
			return err
		}
		file.Raw = raw

		err = hcl.Unmarshal(file.Raw, &file.Unmarshalled)
		if err != nil {
			return err
		}
	}
	return nil
}

func AppendIfMissing(slice []string, i string) []string {
	for _, ele := range slice {
		if ele == i {
			return slice
		}
	}
	return append(slice, i)
}

func (ws *Workspace) getInputs() error {
	r := regexp.MustCompile(`\${data\.terraform_remote_state\.(?P<rs>[a-zA-Z0-9_-]*)\.(?P<var>[a-zA-Z0-9_-]*)}`)

	for filename, file := range ws.Files {
		matches := r.FindAllSubmatch(file.Raw, -1)
		for _, match := range matches {
			if len(match) != 3 {
				continue
			}

			fullName := string(match[0])
			rsName := string(match[1])
			varName := string(match[2])

			var depRef *RemoteState

			for i, dep := range ws.Dependencies {
				if dep.Name == rsName {
					depRef = &ws.Dependencies[i]
				}
			}

			inputExists := false
			for i, input := range ws.Inputs {
				if depRef != nil && varName == input.Name && depRef.equals(*input.Dependency) {
					inputExists = true
					ws.Inputs[i].InFile = AppendIfMissing(ws.Inputs[i].InFile, filename)
				}
			}

			if !inputExists {
				input := Input{
					Name:       varName,
					FullName:   fullName,
					InFile:     []string{filename},
					Dependency: depRef,
					BelongsTo:  ws,
				}

				ws.Inputs = append(ws.Inputs, input)
			}
		}
	}
	return nil
}

func (ws *Workspace) getRemoteState() error {
	for filename, file := range ws.Files {
		bucket, err := jmespath.Search("terraform[0].backend[0].s3[0].bucket", file.Unmarshalled)
		if err != nil {
			return err
		}

		key, err := jmespath.Search("terraform[0].backend[0].s3[0].key", file.Unmarshalled)
		if err != nil {
			return err
		}

		profile, err := jmespath.Search("terraform[0].backend[0].s3[0].profile", file.Unmarshalled)
		if err != nil {
			return err
		}

		region, err := jmespath.Search("terraform[0].backend[0].s3[0].region", file.Unmarshalled)
		if err != nil {
			return err
		}

		if bucket != nil && region != nil && key != nil && profile != nil {
			ws.RemoteState = RemoteState{
				InFile:  filename,
				Bucket:  bucket.(string),
				Key:     key.(string),
				Profile: profile.(string),
				Region:  region.(string),
			}
		}
	}

	return nil
}

func (ws *Workspace) getDependencies() error {
	ws.Dependencies = []RemoteState{}
	for filename, file := range ws.Files {
		remoteStateData, err := jmespath.Search("data[].terraform_remote_state[]", file.Unmarshalled)
		if err != nil {
			return err
		} else if remoteStateData == nil {
			continue
		}

		for _, elem := range remoteStateData.([]interface{}) {
			for k, v := range elem.(map[string]interface{}) {
				bucket, err := jmespath.Search("[0].config[0].bucket", v)
				if err != nil {
					return err
				}

				key, err := jmespath.Search("[0].config[0].key", v)
				if err != nil {
					return err
				}

				profile, err := jmespath.Search("[0].config[0].profile", v)
				if err != nil {
					return err
				}

				region, err := jmespath.Search("[0].config[0].region", v)
				if err != nil {
					return err
				}

				if bucket == nil || region == nil || key == nil || profile == nil {
					continue
				}

				rs := RemoteState{
					InFile:  filename,
					Name:    k,
					Bucket:  bucket.(string),
					Key:     key.(string),
					Profile: profile.(string),
					Region:  region.(string),
				}

				ws.Dependencies = append(ws.Dependencies, rs)
			}
		}
	}
	return nil
}

func (ws *Workspace) getOutputs() error {
	for filename, file := range ws.Files {
		outputs, err := jmespath.Search("output[]", file.Unmarshalled)
		if err != nil {
			return err
		} else if outputs == nil {
			continue
		}

		for _, elem := range outputs.([]interface{}) {
			for k, v := range elem.(map[string]interface{}) {
				o := Output{
					Name:      k,
					Value:     v,
					InFile:    filename,
					BelongsTo: ws,
				}
				ws.Outputs = append(ws.Outputs, o)
			}
		}
	}

	return nil
}
