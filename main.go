package main

import (
	"github.com/binChris/mirror/config"
	"github.com/binChris/mirror/console"
	"github.com/binChris/mirror/mirror"
)

func main() {
	defer console.Cleanup()
	cfg, parallel := config.FromCommandLine()
	mirror.Run(cfg, parallel, console.New())
}
