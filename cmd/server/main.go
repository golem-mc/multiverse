// Command server is the golem server with Multiverse built in.
package main

import (
	"github.com/golem-mc/golem/app"
	_ "github.com/golem-mc/multiverse"
	_ "github.com/golem-mc/multiverse/inventories"
	_ "github.com/golem-mc/multiverse/netherportals"
	_ "github.com/golem-mc/multiverse/portals"
)

func main() { app.Main() }
