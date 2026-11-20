// Command server is the golem server with Multiverse built in.
package main

import (
	"github.com/golem-mc/golem/app"
	_ "github.com/golem-mc/multiverse"
	_ "github.com/golem-mc/multiverse/inventories"
)

func main() { app.Main() }
