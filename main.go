package main

import (
	"os"

	"github.com/jasondellaluce/synchro/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
