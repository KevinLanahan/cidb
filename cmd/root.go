package cmd

import (
	"fmt"
	"os"

	"github.com/KevinLanahan/cidb/runner"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cidb",
	Short: "cidb — step-through debugger for CI pipelines",
	Long: `cidb lets you run CI pipelines locally (GitHub Actions and GitLab CI),
pausing before each step so you can inspect, skip, retry, or
drop into a live shell inside the running container.`,
}

var runCmd = &cobra.Command{
	Use:   "run [workflow-file]",
	Short: "Run a workflow step-by-step",
	Long: `Parses the given workflow file (or auto-discovers .gitlab-ci.yml or .github/workflows/)
and runs each step inside a local Docker container, pausing for your input before each one.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workflowFile := ""
		if len(args) > 0 {
			workflowFile = args[0]
		}
		return runner.Run(workflowFile)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(runCmd)
}
