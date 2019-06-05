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

		rs, err := workspace.getRemoteState()
		if err != nil {
			return workspaces, err
		}
		workspace.RemoteState = rs

		td, err := workspace.getTerraformDependencies()
		if err != nil {
			return workspaces, err
		}
		workspace.Dependencies = append(workspace.Dependencies, td...)

		ti, err := workspace.getTerraformInputs()
		if err != nil {
			return workspaces, err
		}
		workspace.Inputs = append(workspace.Inputs, ti...)

		o, err := workspace.getOutputs()
		if err != nil {
			return workspaces, err
		}
		workspace.Outputs = append(workspace.Outputs, o...)
	}

	// fetch manual info per workspace
	for _, workspace := range workspaces {
		pre, err := workspace.getManual(preFileName)
		if err != nil {
			return workspaces, err
		}
		workspace.PreManual = pre

		post, err := workspace.getManual(postFileName)
		if err != nil {
			return workspaces, err
		}
		workspace.PostManual = post

		md, err := workspace.getManualDependencies(workspaces)
		if err != nil {
			return workspaces, err
		}
		workspace.Dependencies = append(workspace.Dependencies, md...)

		mi, err := workspace.getManualInputs(workspaces)
		if err != nil {
			return workspaces, err
		}
		workspace.Inputs = append(workspace.Inputs, mi...)
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

type Workspace struct {
	Files              map[string]*File `json:"-"`
	Root               string           `json:"root"`
	RemoteState        RemoteState      `json:"remote_state"`
	Dependencies       []RemoteState    `json:"dependencies"`
	Inputs             []Input          `json:"inputs"`
	Outputs            []Output         `json:"outputs"`
	PreManual          Manual           `json:"pre_manual"`
	PreManualRendered  string           `json:"pre_manual_rendered"`
	PostManual         Manual           `json:"post_manual"`
	PostManualRendered string           `json:"post_manual_rendered"`
	graphElement       *dot.Graph
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

func (ws Workspace) getTerraformInputs() ([]Input, error) {
	inputs := []Input{}
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
			for i, input := range inputs {
				if depRef != nil && varName == input.Name && depRef.equals(*input.Dependency) {
					inputExists = true
					inputs[i].InFile = appendIfMissing(inputs[i].InFile, filename)
				}
			}

			if !inputExists {
				input := Input{
					Name:       varName,
					FullName:   fullName,
					InFile:     []string{filename},
					Dependency: depRef,
					BelongsTo:  &ws,
				}

				inputs = append(inputs, input)
			}
		}
	}
	return inputs, nil
}

func (ws Workspace) getRemoteState() (RemoteState, error) {
	rs := RemoteState{}
	for filename, file := range ws.Files {
		bucket, err := jmespath.Search("terraform[0].backend[0].s3[0].bucket", file.Unmarshalled)
		if err != nil {
			return rs, err
		}

		key, err := jmespath.Search("terraform[0].backend[0].s3[0].key", file.Unmarshalled)
		if err != nil {
			return rs, err
		}

		profile, err := jmespath.Search("terraform[0].backend[0].s3[0].profile", file.Unmarshalled)
		if err != nil {
			return rs, err
		}

		region, err := jmespath.Search("terraform[0].backend[0].s3[0].region", file.Unmarshalled)
		if err != nil {
			return rs, err
		}

		if bucket != nil && region != nil && key != nil && profile != nil {
			rs = RemoteState{
				InFile:  filename,
				Bucket:  bucket.(string),
				Key:     key.(string),
				Profile: profile.(string),
				Region:  region.(string),
			}
		}
	}

	return rs, nil
}

func (ws Workspace) getTerraformDependencies() ([]RemoteState, error) {
	d := []RemoteState{}
	for filename, file := range ws.Files {
		remoteStateData, err := jmespath.Search("data[].terraform_remote_state[]", file.Unmarshalled)
		if err != nil {
			return d, err
		} else if remoteStateData == nil {
			continue
		}

		for _, elem := range remoteStateData.([]interface{}) {
			for k, v := range elem.(map[string]interface{}) {
				bucket, err := jmespath.Search("[0].config[0].bucket", v)
				if err != nil {
					return d, err
				}

				key, err := jmespath.Search("[0].config[0].key", v)
				if err != nil {
					return d, err
				}

				profile, err := jmespath.Search("[0].config[0].profile", v)
				if err != nil {
					return d, err
				}

				region, err := jmespath.Search("[0].config[0].region", v)
				if err != nil {
					return d, err
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

				d = append(d, rs)
			}
		}
	}
	return d, nil
}

func (ws Workspace) getManualDependencies(workspaces map[string]*Workspace) ([]RemoteState, error) {
	d := []RemoteState{}
	manuals := map[string]Manual{
		preFileName:  ws.PreManual,
		postFileName: ws.PostManual,
	}
	re := regexp.MustCompile(`\{\{.*\}\}`)
	for filename, m := range manuals {
		submaches := re.FindAllStringSubmatch(string(m), -1)
		for _, sm := range submaches {
			match := sm[0]
			seg := strings.SplitN(strings.Trim(match, "{{}}"), ".", 2)
			if len(seg) != 2 {
				return d, fmt.Errorf("Reference '%s' seems to be malformed\n", match)
			}

			workspacePath := seg[0]

			for name, workspace := range workspaces {
				if strings.Contains(name, workspacePath) {

					rs := RemoteState{
						InFile:  filename,
						Name:    workspacePath,
						Bucket:  workspace.RemoteState.Bucket,
						Key:     workspace.RemoteState.Key,
						Profile: workspace.RemoteState.Profile,
						Region:  workspace.RemoteState.Region,
					}

					d = append(d, rs)

				}
			}
		}
	}
	return d, nil

}

func (ws Workspace) getManualInputs(workspaces map[string]*Workspace) ([]Input, error) {
	inputs := []Input{}

	manuals := map[string]Manual{
		preFileName:  ws.PreManual,
		postFileName: ws.PostManual,
	}
	re := regexp.MustCompile(`\{\{.*\}\}`)
	for filename, m := range manuals {
		submaches := re.FindAllSubmatch([]byte(m), -1)
		for _, sm := range submaches {
			match := string(sm[0])
			seg := strings.SplitN(strings.Trim(match, "{{}}"), ".", 2)
			if len(seg) != 2 {
				return inputs, fmt.Errorf("Reference '%s' seems to be malformed\n", match)
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
				return inputs, fmt.Errorf("Reference to '%s' in workspace '%s' does not exist\n", outputName, workspacePath)
			}

			input := Input{
				Name:       outputName,
				FullName:   match,
				InFile:     []string{filename},
				Dependency: &o.BelongsTo.RemoteState,
				BelongsTo:  &ws,
			}

			inputs = append(inputs, input)
		}
	}

	return inputs, nil
}

func (ws Workspace) getOutputs() ([]Output, error) {
	o := []Output{}
	for filename, file := range ws.Files {
		outputs, err := jmespath.Search("output[]", file.Unmarshalled)
		if err != nil {
			return o, err
		} else if outputs == nil {
			continue
		}

		for _, elem := range outputs.([]interface{}) {
			for k, v := range elem.(map[string]interface{}) {
				output := Output{
					Name:      k,
					Value:     v,
					InFile:    filename,
					BelongsTo: &ws,
				}
				o = append(o, output)
			}
		}
	}

	return o, nil
}

func (ws Workspace) getManual(filename string) (Manual, error) {
	m := Manual("")
	path := ws.Root + "/" + filename
	if _, err := os.Stat(path); err == nil {
		raw, err := ioutil.ReadFile(path)
		if err != nil {
			return m, err
		}
		m = Manual(raw)
	}
	return m, nil
}

func (m Manual) render(inputs []Input) (string, error) {
	rendered := string(m)

	for _, input := range inputs {
		if strings.HasPrefix(input.FullName, "{{") &&
			strings.HasSuffix(input.FullName, "}}") {
			chdir := input.ReferesTo.BelongsTo.Root
			command := "terraform"
			args := []string{
				"output",
				input.ReferesTo.Name,
			}

			cmd := exec.Command(command, args...)
			cmd.Dir = chdir

			out, err := cmd.Output()
			if err != nil {
				errOut := fmt.Errorf("Could not run command '%s %s' in '%s': %s", command, strings.Join(args, " "), chdir, err.Error())
				return rendered, errOut
			}
			outStr := strings.TrimSpace(string(out))

			rendered = strings.Replace(rendered, input.FullName, outStr, -1)
		}
	}

	return rendered, nil
}
