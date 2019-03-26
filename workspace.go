package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
	}

	return workspaces, err
}

type Workspace struct {
	Files        map[string]*File
	RemoteState  RemoteState
	Dependencies []RemoteState
	Inputs       []Input
}

type File struct {
	Raw          []byte
	Unmarshalled interface{}
}

type Input struct {
	Name        string
	RemoteState *RemoteState
	ReferesTo   *Workspace
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
		fmt.Println(filename)
		matches := r.FindAllSubmatch(file.Raw, -1)
		if len(matches) > 0 {
			fmt.Println("  " + string(matches[0][1]))
			fmt.Println("     " + string(matches[0][2]))
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
