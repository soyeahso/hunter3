package main

import (
	"fmt"
	"os"

	"github.com/tillberg/autorestart"
	"github.com/soyeahso/hunter3/internal/cli"
)

func main() {
	go autorestart.RestartOnChange()

	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
