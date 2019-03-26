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
				workspaces[workspacePath] = &Workspace{Files: files}
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
			found := false
			for _, dep := range workspaces {
				if input.Dependency.equals(dep.RemoteState) {

					for o, output := range dep.Outputs {
						if input.Name == output.Name {
							found = true
							workspace.Inputs[i].ReferesTo = &dep.Outputs[o]
							dep.Outputs[o].ReferedBy = append(dep.Outputs[o].ReferedBy, &workspace.Inputs[i])
						}

					}
				}
			}
			if !found {
				return workspaces, fmt.Errorf("Could not resolve dependencie for %s", input.Name)
			}
		}
	}

	return workspaces, err
}

func RenderWorkspaces(workspaces map[string]*Workspace) *dot.Graph {
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
		if len(workspace.Inputs) > 0 {
			for _, input := range workspace.Inputs {
				if input.ReferesTo != nil {
					g.Edge(input.graphElement, input.ReferesTo.graphElement)
				} else {
					//printJSON(input)
				}
			}
		}
	}

	return g
}

type Workspace struct {
	Files        map[string]*File `json:"-"`
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

func (ws *Workspace) getInputs() error {
	r := regexp.MustCompile(`\${data\.terraform_remote_state\.(?P<rs>[a-zA-Z_-]*)\.(?P<var>[a-zA-Z_-]*)}`)

	for filename, file := range ws.Files {
		matches := r.FindAllSubmatch(file.Raw, -1)
		for _, match := range matches {
			if len(match) != 3 {
				continue
			}

			rsName := string(match[1])
			varName := string(match[2])

			var depRef *RemoteState

			for i, dep := range ws.Dependencies {
				if dep.Name == rsName {
					depRef = &ws.Dependencies[i]
				}
			}

			elemExists := false
			for i, elem := range ws.Inputs {
				if varName == elem.Name && depRef.equals(*elem.Dependency) {
					elemExists = true
					ws.Inputs[i].InFile = append(ws.Inputs[i].InFile, filename)
				}
			}

			if !elemExists {
				input := Input{
					Name:       varName,
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
	ws.Dependencies = []RemoteState{}
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
