// Command fanear reads the records FanEar (饭耳) writes into its save location.
package main

import (
	"os"
	_ "time/tzdata" // FanEar data names IANA zones; keep them resolvable everywhere

	"github.com/Dikevin/fanear-cli/internal/cli"
)

func main() { os.Exit(cli.Main()) }
