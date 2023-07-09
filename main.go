package main

import (
	"github.com/binChris/go-mirror/config"
	"github.com/binChris/go-mirror/console"
	"github.com/binChris/go-mirror/mirror"
)

func main() {
	defer console.Cleanup()
	cfg, parallel := config.FromCommandLine()
	mirror.Run(cfg, parallel, console.New())
}
