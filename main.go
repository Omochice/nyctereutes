// Command nyctereutes is the entry point that dispatches the CLI subcommands.
package main

import (
	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/nyctereutes"
)

func main() {
	cli.Run(nyctereutes.MainCommand)
}
