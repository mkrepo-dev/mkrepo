package cmd

import (
	"fmt"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal"
	"github.com/spf13/cobra"
)

func NewCommand(readme string, license string) *cobra.Command {
	command := &cobra.Command{
		Use: "mkrepo",
	}
	command.AddCommand(&cobra.Command{
		Short: "Prints version information",
		Use:   "version",
		Run: func(cmd *cobra.Command, args []string) {
			version := internal.ReadVersion()
			fmt.Printf("Version: %s\nGo Version: %s\nRevision: %s\nBuild Datetime: %s\n",
				version.Version, version.GoVersion, version.Revision[:7], version.BuildDatetime.Format(time.RFC3339))
		},
	})
	command.AddCommand(&cobra.Command{
		Short: "Prints README",
		Use:   "readme",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(readme)
		},
	})
	command.AddCommand(&cobra.Command{
		Short: "Prints license",
		Use:   "license",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(license)
		},
	})
	command.AddCommand(NewServerCommand())

	return command
}
