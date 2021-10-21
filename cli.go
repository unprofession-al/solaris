package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/russross/blackfriday"
	"github.com/spf13/cobra"
)

type App struct {
	cfg struct {
		// root
		rootBase           string
		rootIgnorePatterns []string
		rootDebug          bool

		// graph
		graphDetailed bool

		// json
		jsonCompact bool

		// plan
		planRoots        []string
		planJSON         bool
		planRenderManual bool
	}

	// entry point
	Execute func() error
}

func NewApp() *App {
	a := &App{}

	// root
	rootCmd := &cobra.Command{
		Use:   "solaris",
		Short: "handle dependencies between multiple terraform workspaces",
	}
	rootCmd.PersistentFlags().BoolVar(&a.cfg.rootDebug, "debug", false, "write debug output to STDERR")
	rootCmd.PersistentFlags().StringVarP(&a.cfg.rootBase, "base", "b", ".", "the base directory")
	rootCmd.PersistentFlags().StringSliceVarP(&a.cfg.rootIgnorePatterns, "ignore", "i", []string{`\.terraform`, "modules"}, "ignore subdirectories that match the given patterns")
	a.Execute = rootCmd.Execute

	// graph
	graphCmd := &cobra.Command{
		Use:   "graph",
		Short: "generate dot output of terraform workspace dependencies",
		Run:   a.graphCmd,
	}
	graphCmd.PersistentFlags().BoolVarP(&a.cfg.graphDetailed, "detailed", "d", false, "draw a detailed graph")
	rootCmd.AddCommand(graphCmd)

	// lint
	lintCmd := &cobra.Command{
		Use:   "lint",
		Short: "lint terraform workspace dependencies",
		Run:   a.lintCmd,
	}
	rootCmd.AddCommand(lintCmd)

	// json
	jsonCmd := &cobra.Command{
		Use:   "json",
		Short: "print a json representation of terraform workspace dependencies",
		Run:   a.jsonCmd,
	}
	jsonCmd.PersistentFlags().BoolVarP(&a.cfg.jsonCompact, "compact", "c", false, "print compact JSON")
	rootCmd.AddCommand(jsonCmd)

	// plan
	planCmd := &cobra.Command{
		Use:   "plan",
		Short: "print execution order of terraform workspaces",
		Run:   a.planCmd,
	}
	planCmd.PersistentFlags().StringSliceVarP(&a.cfg.planRoots, "roots", "r", []string{}, "plan only to execute these workspaces and workspaces depending on them")
	planCmd.PersistentFlags().BoolVarP(&a.cfg.planJSON, "json", "j", false, "print as JSON")
	planCmd.PersistentFlags().BoolVarP(&a.cfg.planRenderManual, "render", "m", false, "Render Pre-/Post manuals (this requires `terraform` to be installed)")
	rootCmd.AddCommand(planCmd)

	// version
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version info",
		Run:   a.versionCmd,
	}
	rootCmd.AddCommand(versionCmd)

	return a
}

func (a *App) debug(out string) {
	if a.cfg.rootDebug {
		fmt.Fprint(os.Stderr, out)
	}
}

func (a *App) graphCmd(cmd *cobra.Command, args []string) {
	workspaces, err := GetWorkspaces(a.cfg.rootBase, a.cfg.rootIgnorePatterns)
	if err != nil {
		log.Fatal(err)
	}
	if a.cfg.graphDetailed {
		graph := RenderWorkspacesDetailed(workspaces)
		fmt.Println(graph.String())
	} else {
		graph := RenderWorkspaces(workspaces)
		fmt.Println(graph.String())
	}
	fmt.Printf("\n/*\n   Use 'solaris ... | fdp -Tsvg > out.svg' or\n   similar to generate a vector visualization\n*/\n")
}

func (a *App) lintCmd(cmd *cobra.Command, args []string) {
	workspaces, err := GetWorkspaces(a.cfg.rootBase, a.cfg.rootIgnorePatterns)
	if err != nil {
		log.Fatal(err)
	}

	errs := Lint(workspaces)
	for k, v := range errs {
		fmt.Printf("%s:\n", k)
		for _, e := range v {
			fmt.Printf("   %s\n", e)
		}
	}
}

func (a *App) jsonCmd(cmd *cobra.Command, args []string) {
	workspaces, err := GetWorkspaces(a.cfg.rootBase, a.cfg.rootIgnorePatterns)
	if err != nil {
		log.Fatal(err)
	}

	var out []byte
	if a.cfg.jsonCompact {
		out, err = json.Marshal(workspaces)
	} else {
		out, err = json.MarshalIndent(workspaces, "", "    ")
	}
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))
}

func (a *App) planCmd(cmd *cobra.Command, args []string) {
	workspaces, err := GetWorkspaces(a.cfg.rootBase, a.cfg.rootIgnorePatterns)
	if err != nil {
		log.Fatal(err)
	}
	wsarray := []*Workspace{}
	for _, ws := range workspaces {
		wsarray = append(wsarray, ws)
	}

	plan, err := BuildExecutionPlan(wsarray, a.cfg.planRoots, a.debug)
	if err != nil {
		log.Fatal(err)
	}

	for tier, workspaces := range plan {
		for i, ws := range workspaces {
			if ws.PreManual != "" {
				manual := ""
				if a.cfg.planRenderManual {
					manual, err = ws.PreManual.render(ws.Inputs)
					if err != nil {
						log.Fatal(err)
					}
				} else {
					manual = string(ws.PreManual)
				}
				x := blackfriday.Run([]byte(manual))
				plan[tier][i].PreManualRendered = string(x)
			}
			if ws.PostManual != "" {
				manual := ""
				if a.cfg.planRenderManual {
					manual, err = ws.PostManual.render(ws.Inputs)
					if err != nil {
						log.Fatal(err)
					}
				} else {
					manual = string(ws.PostManual)
				}

				x := blackfriday.Run([]byte(manual))
				plan[tier][i].PostManualRendered = string(x)
			}
		}
	}

	if a.cfg.planJSON {
		out, err := json.MarshalIndent(plan, "", "    ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(out))
	} else {
		out := RenderExecutionPlanAsHTML(plan)
		fmt.Println(out)
	}

}

func (a *App) versionCmd(cmd *cobra.Command, args []string) {
	fmt.Println(versionInfo())
}
