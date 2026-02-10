package main

import (
	"os"

	"github.com/tillberg/autorestart"
	"github.com/soyeahso/hunter3/internal/cli"
)

func main() {
	go autorestart.RestartOnChange()

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
