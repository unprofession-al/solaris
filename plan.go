package main

import (
	"fmt"
	"strings"
)

func BuildExecutionPlan(workspaces []*Workspace, roots []string) ([][]*Workspace, error) {
	plan := [][]*Workspace{}
	if len(roots) == 0 {
		// root are all workspaces which do not depend on anything
		for _, workspace := range workspaces {
			if len(workspace.Inputs) == 0 {
				roots = append(roots, workspace.Root)
			}
		}
	} else {
		// reduce workspaces to only the ones that depend on the give roots
		var getRelevantWorkspaces func(*Workspace) []*Workspace
		getRelevantWorkspaces = func(ws *Workspace) []*Workspace {
			out := []*Workspace{ws}
			for _, output := range ws.Outputs {
				for _, input := range output.ReferedBy {
					out = append(out, getRelevantWorkspaces(input.BelongsTo)...)
				}
			}
			return out
		}

		relevantWorkspaces := []*Workspace{}
		for _, root := range roots {
			exists := false
			for _, ws := range workspaces {
				if strings.Contains(ws.Root, root) {
					exists = true
					relevantWorkspaces = append(relevantWorkspaces, getRelevantWorkspaces(ws)...)
				}
			}
			if !exists {
				err := fmt.Errorf("Workspace '%s' does not exist", root)
				return plan, err
			}
		}
		workspaces = relevantWorkspaces

		// remove roots that depend on workspaces that are considered
		newRoots := []string{}
		for _, root := range roots {
			hasDependencies := false
			for _, ws := range workspaces {
				if strings.Contains(ws.Root, root) {
					for _, input := range ws.Inputs {
						for _, wrkspce := range workspaces {
							if input.ReferesTo.BelongsTo.Root == wrkspce.Root {
								hasDependencies = true
							}
						}
					}
				}
			}
			if !hasDependencies {
				newRoots = append(newRoots, root)
			}
		}
		roots = newRoots
	}

	firstTier := []*Workspace{}
	for _, root := range roots {
		for _, ws := range workspaces {
			if strings.Contains(ws.Root, root) {
				firstTier = append(firstTier, ws)
			}
		}
	}
	plan = append(plan, firstTier)

	var nextTier func(plan [][]*Workspace, workspaces []*Workspace) [][]*Workspace
	nextTier = func(plan [][]*Workspace, workspaces []*Workspace) [][]*Workspace {
		// get a list of all workspaces that already have been planned
		planned := []*Workspace{}
		for _, i := range plan {
			for _, j := range i {
				planned = append(planned, j)
			}
		}

		// get a list of uniq unplanned workspaces
		unplanned := []*Workspace{}
		for _, i := range workspaces {
			isPlanned := false
			for _, j := range planned {
				if i.RemoteState.equals(j.RemoteState) {
					isPlanned = true
				}
			}
			if !isPlanned {
				exists := false
				for _, y := range unplanned {
					if i.RemoteState.equals(y.RemoteState) {
						exists = true
					}
				}
				if !exists {
					unplanned = append(unplanned, i)
				}
			}
		}

		// exit if all is planned
		if len(unplanned) < 1 {
			return plan
		}

		// plan all unplanned with satisfied dependencies
		next := []*Workspace{}
		for _, ws := range unplanned {
			toSatisfy := 0
			for _, dep := range ws.Dependencies {
				for _, i := range workspaces {
					if dep.equals(i.RemoteState) {
						toSatisfy++
						break
					}
				}
			}
			satisfied := 0
			for _, dep := range ws.Dependencies {
				for _, p := range planned {
					if dep.equals(p.RemoteState) {
						satisfied++
					}
				}
			}
			if satisfied == toSatisfy {
				next = append(next, ws)
			}
		}

		plan = append(plan, next)
		return nextTier(plan, workspaces)
	}

	return nextTier(plan, workspaces), nil
}
