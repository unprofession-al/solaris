package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/emicklei/dot"
	"github.com/hashicorp/hcl"
	jmespath "github.com/jmespath/go-jmespath"
)

const (
	tfext        = ".tf"
	preFileName  = "PreManual.md"
	postFileName = "PostManual.md"
)

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

		err = workspace.getManual()
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

	// get relation betwenn workspaces and manual work
	for _, workspace := range workspaces {
		out, err := workspace.PostManual.Render(workspaces)
		if err != nil {
			return workspaces, err
		}
		fmt.Println(out)
	}

	return workspaces, err
}

type Workspace struct {
	Files        map[string]*File `json:"-"`
	Root         string           `json:"root"`
	RemoteState  RemoteState      `json:"remote_state"`
	Dependencies []RemoteState    `json:"dependencies"`
	Inputs       []Input          `json:"inputs"`
	Outputs      []Output         `json:"outputs"`
	PreManual    Manual           `json:"PreManual"`
	PostManual   Manual           `json:"PostManual"`
	graphElement *dot.Graph
}

type Manual string

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

func (ws *Workspace) getInputs() error {
	r := regexp.MustCompile(`\${data\.terraform_remote_state\.(?P<rs>[a-zA-Z0-9_-]*)\.(?P<var>[a-zA-Z0-9_-]*)}`)

	appendIfMissing := func(slice []string, i string) []string {
		for _, ele := range slice {
			if ele == i {
				return slice
			}
		}
		return append(slice, i)
	}

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
					ws.Inputs[i].InFile = appendIfMissing(ws.Inputs[i].InFile, filename)
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

func (ws *Workspace) getManual() error {
	prePath := ws.Root + "/" + preFileName
	if _, err := os.Stat(prePath); err == nil {
		raw, err := ioutil.ReadFile(prePath)
		if err != nil {
			return err
		}
		ws.PreManual = Manual(raw)
	}

	postPath := ws.Root + "/" + postFileName
	if _, err := os.Stat(postPath); err == nil {
		raw, err := ioutil.ReadFile(postPath)
		if err != nil {
			return err
		}
		ws.PostManual = Manual(raw)
	}
	return nil
}

func (m Manual) GetReferences(workspaces map[string]*Workspace) (map[string]*Output, error) {
	out := map[string]*Output{}
	re := regexp.MustCompile(`\{\{.*\}\}`)
	submaches := re.FindAllSubmatch([]byte(m), -1)
	for _, sm := range submaches {
		match := string(sm[0])
		seg := strings.SplitN(strings.Trim(match, "{{}}"), ".", 2)
		if len(seg) != 2 {
			return out, fmt.Errorf("Reference '%s' seems to be malformed\n", match)
		}

		workspacePath := seg[0]
		outputName := seg[1]

		var o *Output
		for name, workspace := range workspaces {
			if strings.Contains(name, workspacePath) {
				for _, output := range workspace.Outputs {
					if output.Name == outputName {
						o = &output
					}
				}
			}
		}
		if o == nil {
			return out, fmt.Errorf("Reference to '%s' in workspace '%s' does not exist\n", outputName, workspacePath)
		}

		out[match] = o
	}
	return out, nil
}

func (m Manual) Render(workspaces map[string]*Workspace) (string, error) {
	rendered := string(m)

	references, err := m.GetReferences(workspaces)
	if err != nil {
		errOut := fmt.Errorf("Could not get references: %s", err.Error())
		return rendered, errOut
	}
	for reference, output := range references {
		chdir := output.BelongsTo.Root
		command := "terraform"
		args := []string{
			"output",
			output.Name,
		}

		cmd := exec.Command(command, args...)
		cmd.Dir = chdir

		out, err := cmd.Output()
		if err != nil {
			errOut := fmt.Errorf("Could not run command '%s %s' in '%s': %s", command, strings.Join(args, " "), chdir, err.Error())
			return rendered, errOut
		}

		rendered = strings.Replace(rendered, reference, string(out), -1)
	}

	return rendered, nil
}
