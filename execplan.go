package main

import "fmt"

func BuildExecutionPlan(workspaces map[string]*Workspace, roots []string) ([][]string, error) {
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
			ws, ok := workspaces[root]
			if ok {
				relevantWorkspaces = append(relevantWorkspaces, getRelevantWorkspaces(ws)...)
			} else {
				err := fmt.Errorf("Workspace '%s' does not exist", root)
				return [][]string{}, err
			}
		}

		workspaces = make(map[string]*Workspace)
		for _, ws := range relevantWorkspaces {
			workspaces[ws.Root] = ws
		}

		// remove roots that depend on workspaces that are considered
		newRoots := []string{}
		for _, root := range roots {
			hasDependencies := false
			ws, ok := workspaces[root]
			if ok {
				for _, input := range ws.Inputs {
					for name := range workspaces {
						if input.ReferesTo.BelongsTo.Root == name {
							hasDependencies = true
						}
					}
				}
			} else {
				err := fmt.Errorf("Workspace '%s' does not exist", root)
				return [][]string{}, err
			}
			if !hasDependencies {
				newRoots = append(newRoots, root)
			}
		}
		roots = newRoots
	}

	plan := [][]string{}
	plan = append(plan, roots)

	var nextTier func(plan [][]string, workspaces map[string]*Workspace) ([][]string, error)
	nextTier = func(plan [][]string, workspaces map[string]*Workspace) ([][]string, error) {
		// get a list of all workspaces that already have been planned
		planned := []string{}
		for _, i := range plan {
			for _, j := range i {
				planned = append(planned, j)
			}
		}

		// get a list of all workspaces planned in the last step
		latest := plan[len(plan)-1]

		// get a list of inputs that refer to the last planned steps
		upcomingInputs := []*Input{}
		for _, name := range latest {
			ws := workspaces[name]
			for _, output := range ws.Outputs {
				upcomingInputs = append(upcomingInputs, output.ReferedBy...)
			}
		}

		// get a list of workspaces which follow the last planned steps
		possibleNext := map[string]*Workspace{}
		for _, input := range upcomingInputs {
			possibleNext[input.BelongsTo.Root] = input.BelongsTo
		}

		// check if requirements are fulfilled and append to next
		next := []string{}
		for _, ws := range possibleNext {
			fulfilled := 0
			for _, input := range ws.Inputs {
				for _, p := range planned {
					if input.ReferesTo == nil {
						err := fmt.Errorf("Unresolvable dependency '%s' please fix in %s (file %s)", input.FullName, input.BelongsTo.Root, input.InFile)
						return [][]string{}, err
					} else if _, ok := workspaces[input.ReferesTo.BelongsTo.Root]; !ok {
						fulfilled++
					} else if p == input.ReferesTo.BelongsTo.Root {
						fulfilled++
					}
				}
			}
			if len(ws.Inputs) == fulfilled {
				next = append(next, ws.Root)

			}
		}

		if len(next) > 0 {
			plan = append(plan, next)
			return nextTier(plan, workspaces)
		} else {
			return plan, nil
		}
	}

	return nextTier(plan, workspaces)
}
