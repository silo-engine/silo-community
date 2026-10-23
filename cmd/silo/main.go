//go:build linux

package main

import "github.com/silo-engine/silo-community/ui/cli"

func main() {

	siloCLI := cli.NewSiloCLI()
	siloCLI.Run()
}
