// Entry point of the nyctereutes CLI, which drives the glab toolchain.
package main

import (
	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/nyctereutes"
)

func main() {
	cli.Run(nyctereutes.MainCommand)
}
