//go:build linux

package main

import "github.com/silo-engine/silo-community/ui/cli"

func main() {
	cli.NewSiloCLI().Run()
}
