package cmd

import (
	"fmt"
	"os"

	"github.com/KevinLanahan/lokal/runner"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <slug>",
	Short: "Delete a shared session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]
		runner.LoadEnvForDelete()

		fmt.Printf("  Delete session %s? [y/N] > ", slug)
		var confirm string
		fmt.Fscan(os.Stdin, &confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("  Cancelled.")
			return nil
		}

		if err := runner.DeleteSession(slug); err != nil {
			return err
		}

		fmt.Printf("  Session %s deleted.\n", slug)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
