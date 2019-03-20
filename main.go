package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/emicklei/dot"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/terraform/config"
	"github.com/tidwall/gjson"
)

var root string
var format string
var experimental string

const tfext = ".tf"

func init() {
	flag.StringVar(&root, "r", ".", "root path")
	flag.StringVar(&format, "f", "dot", "output format ('dot' or 'text')")
	flag.StringVar(&experimental, "e", "false", "experimental mode")
}

func getFileList(root string) (map[string][]string, error) {
	dirs := make(map[string][]string)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, "template") {
			return nil
		}
		if filepath.Ext(path) == tfext {
			dir, file := filepath.Split(path)
			if files, found := dirs[dir]; found {
				files = append(files, file)
				dirs[dir] = files
			} else {
				dirs[dir] = []string{file}
			}
		}
		return nil
	})

	return dirs, err
}

func readFile(dir, file string) ([]byte, error) {
	var data interface{}

	content, err := ioutil.ReadFile(dir + file)
	if err != nil {
		return []byte{}, err
	}

	err = hcl.Unmarshal(content, &data)
	if err != nil {
		return []byte{}, err
	}

	return json.Marshal(data)
}

type RemoteState struct {
	InFile  string
	Name    string
	Bucket  string
	Key     string
	Profile string
	Region  string
}

func (orig RemoteState) equals(other RemoteState) bool {
	if orig.Bucket == other.Bucket &&
		orig.Key == other.Key &&
		orig.Profile == other.Profile &&
		orig.Region == other.Region {
		return true
	}
	return false
}

func getRemoteState(file string, in []byte) (RemoteState, error) {
	out := RemoteState{}

	type raw [][]struct {
		S3 []struct {
			Bucket        string `json:"bucket"`
			DynamodbTable string `json:"dynamodb_table"`
			Encrypt       bool   `json:"encrypt"`
			Key           string `json:"key"`
			Profile       string `json:"profile"`
			Region        string `json:"region"`
		} `json:"s3"`
	}

	value := gjson.GetBytes(in, "terraform.#.backend")
	if value.Exists() {
		var data raw
		err := json.Unmarshal([]byte(value.String()), &data)
		if err != nil {
			return out, err
		}

		for _, i := range data {
			for _, j := range i {
				v := j.S3
				if len(v) > 0 {
					rs := RemoteState{
						InFile:  file,
						Bucket:  v[0].Bucket,
						Key:     v[0].Key,
						Profile: v[0].Profile,
						Region:  v[0].Region,
					}
					out = rs
				}
			}
		}
	}
	return out, nil
}

func getDependencies(file string, in []byte) ([]RemoteState, error) {
	out := []RemoteState{}

	type raw [][]map[string][]struct {
		Backend string `json:"backend"`
		Config  []struct {
			Bucket  string `json:"bucket"`
			Key     string `json:"key"`
			Profile string `json:"profile"`
			Region  string `json:"region"`
		} `json:"config"`
	}

	value := gjson.GetBytes(in, "data.#.terraform_remote_state")
	if value.Exists() {
		var data raw
		err := json.Unmarshal([]byte(value.String()), &data)
		if err != nil {
			return out, err
		}

		for _, i := range data {
			for _, j := range i {
				for k, v := range j {
					if len(v) > 0 {
						y := v[0].Config
						if len(y) > 0 {
							x := y[0]
							rs := RemoteState{
								InFile:  file,
								Name:    k,
								Bucket:  x.Bucket,
								Key:     x.Key,
								Profile: x.Profile,
								Region:  x.Region,
							}
							out = append(out, rs)
						}
					}
				}
			}
		}
	}
	return out, nil
}

type Terraform struct {
	State        RemoteState
	Dependencies []RemoteState
}

func main() {
	flag.Parse()

	dirs, err := getFileList(root)
	if err != nil {
		panic(err)
	}

	terraforms := map[string]Terraform{}

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
		terraforms[strings.TrimPrefix(dir, root)] = tf
	}

	g := dot.NewGraph(dot.Directed)
	nodes := map[string]dot.Node{}

	for name, _ := range terraforms {
		nodes[name] = g.Node(name)
	}

	if experimental == "false" {
		for name, tf := range terraforms {
			if len(tf.Dependencies) > 0 {
				if format == "text" {
					fmt.Println(name)
				}
				for _, dep := range tf.Dependencies {
					for otherName, otherTf := range terraforms {
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
	} else {
		fmt.Println("ok nice")
		c, err := config.LoadDir(root)
		if err != nil {
			panic(err)
		}

		for _, r := range c.Resources {
			if r.Type == "terraform_remote_state" {
				spew.Dump(r)
				fmt.Println(r.(*RawConfig)["config"])
				fmt.Println("----")
			}
		}
	}
}
