package main

import (
	"fmt"
	"os"

	_ "embed"

	"github.com/mkrepo-dev/mkrepo/cmd"
)

//go:embed README.md
var readme string

//go:embed LICENSE
var license string

func main() {
	command := cmd.NewCommand(readme, license)
	err := command.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot execute command: %v\n", err)
		os.Exit(1)
	}
}
