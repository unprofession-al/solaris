package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var (
	rootBase           string
	rootIgnorePatterns []string

	graphDetailed bool

	jsonCompact bool

	planRoots []string
	planJSON  bool
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&rootBase, "base", "b", ".", "the base directory")
	rootCmd.PersistentFlags().StringSliceVarP(&rootIgnorePatterns, "ignore", "i", []string{}, "ignore subdirectories that match the given patterns")

	rootCmd.AddCommand(graphCmd)
	graphCmd.PersistentFlags().BoolVarP(&graphDetailed, "detailed", "d", false, "draw a detailed graph")

	rootCmd.AddCommand(lintCmd)

	rootCmd.AddCommand(jsonCmd)
	jsonCmd.PersistentFlags().BoolVarP(&jsonCompact, "compact", "c", false, "print compact JSON")

	rootCmd.AddCommand(planCmd)
	planCmd.PersistentFlags().StringSliceVarP(&planRoots, "roots", "r", []string{}, "plan only to execute these workspaces and workspaces depending on them")
	planCmd.PersistentFlags().BoolVarP(&planJSON, "json", "j", false, "print as JSON")

}

var rootCmd = &cobra.Command{
	Use:   "solaris",
	Short: "handle dependencies between multiple terraform workspaces",
}

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "generate dot output of terraform workspace dependencies",
	Run: func(cmd *cobra.Command, args []string) {
		workspaces, err := GetWorkspaces(rootBase, rootIgnorePatterns)
		if err != nil {
			log.Fatal(err)
		}
		if graphDetailed {
			graph := RenderWorkspacesDetailed(workspaces)
			fmt.Println(graph.String())
		} else {
			graph := RenderWorkspaces(workspaces)
			fmt.Println(graph.String())
		}
		fmt.Printf("\n/*\n   Use 'solaris ... | fdp -Tsvg > out.svg' or\n   similar to generate a vector visualization\n*/\n")
	},
}

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "lint terraform workspace dependencies",
	Run: func(cmd *cobra.Command, args []string) {
		workspaces, err := GetWorkspaces(rootBase, rootIgnorePatterns)
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
	},
}

var jsonCmd = &cobra.Command{
	Use:   "json",
	Short: "print a json representation of terraform workspace dependencies",
	Run: func(cmd *cobra.Command, args []string) {
		workspaces, err := GetWorkspaces(rootBase, rootIgnorePatterns)
		if err != nil {
			log.Fatal(err)
		}

		var out []byte
		if jsonCompact {
			out, err = json.Marshal(workspaces)
		} else {
			out, err = json.MarshalIndent(workspaces, "", "    ")
		}
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(string(out))
	},
}

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "print execution order of terraform workspaces",
	Run: func(cmd *cobra.Command, args []string) {
		workspaces, err := GetWorkspaces(rootBase, rootIgnorePatterns)
		if err != nil {
			log.Fatal(err)
		}

		plan, err := BuildExecutionPlan(workspaces, planRoots)
		if err != nil {
			log.Fatal(err)
		}
		if planJSON {
			out, err := json.MarshalIndent(plan, "", "    ")
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(string(out))
		} else {
			for tier, workspaces := range plan {
				fmt.Printf("Tier %d:\n", tier)
				for _, ws := range workspaces {
					fmt.Printf("   %s\n", ws)
				}
			}
		}

	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
