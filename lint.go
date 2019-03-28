package main

import (
	"fmt"
	"strings"
)

func Lint(workspaces map[string]*Workspace) map[string][]string {
	out := map[string][]string{}

	outputErrors := lintUnusedOuputs(workspaces)
	if len(outputErrors) > 0 {
		out["Usused Outputs"] = outputErrors
	}

	inputErrors := lintInexistentInputs(workspaces)
	if len(inputErrors) > 0 {
		out["Inexistent Inputs"] = inputErrors
	}

	dataSourceErrors := lintUnusedRemoteStateDataSources(workspaces)
	if len(dataSourceErrors) > 0 {
		out["Unused terraform_remote_state data sources"] = dataSourceErrors
	}

	circularErrors := lintCirularDependencies(workspaces)
	if len(circularErrors) > 0 {
		out["Circular Dependencies"] = circularErrors
	}

	return out
}

func lintUnusedOuputs(workspaces map[string]*Workspace) []string {
	errs := []string{}
	for name, workspace := range workspaces {
		for _, output := range workspace.Outputs {
			if len(output.ReferedBy) == 0 {
				errs = append(errs, fmt.Sprintf("output '%s' of workspace '%s' (in file '%s') seems to be unused", output.Name, name, output.InFile))
			}
		}
	}
	return errs
}

func lintInexistentInputs(workspaces map[string]*Workspace) []string {
	errs := []string{}
	for name, workspace := range workspaces {
		for _, input := range workspace.Inputs {
			if input.ReferesTo == nil {
				errs = append(errs, fmt.Sprintf("input '%s' of workspace '%s' (in file '%s') seems refer to an inexistent output", input.FullName, name, input.InFile))
			}
		}
	}
	return errs
}

func lintUnusedRemoteStateDataSources(workspaces map[string]*Workspace) []string {
	errs := []string{}
	for name, workspace := range workspaces {
		for _, dep := range workspace.Dependencies {
			depUsed := false
			for _, input := range workspace.Inputs {
				if input.Dependency != nil && input.Dependency.equals(dep) {
					depUsed = true
				}
			}
			if !depUsed {
				errs = append(errs, fmt.Sprintf("terraform_remote_state data source '%s' in workspace '%s' (in file '%s') seems to be unused", dep.Name, name, dep.InFile))
			}
		}
	}
	return errs
}

func lintCirularDependencies(workspaces map[string]*Workspace) []string {
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
	errs := []string{}
	for name, workspace := range workspaces {
		err := checkCircular(workspace, name, []string{})

		if err != nil {
			errs = append(errs, fmt.Sprintf("circular dependency in workspace '%s': '%s'", name, err))
		}
	}
	return errs
}
