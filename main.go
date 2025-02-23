package main

import (
	"fmt"
	"os"

	"github.com/maestro-go/maestro/internal/cli"
)

func main() {
	rootCmd := cli.SetupRootCommand()

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(0)
}
