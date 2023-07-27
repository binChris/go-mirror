package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

type Config struct {
	Source        string
	Destination   string
	CreateDir     rune
	DeleteDir     rune
	CreateFile    rune
	OverwriteFile rune
	DeleteFile    rune
}

func FromCommandLine() (Config, int) {
	var cfg Config
	parallel := 5
	flag.ErrHelp = errors.New("usage: mirror (flags) (source dir) (target dir)")
	flag.IntVar(&parallel, "parallel", parallel, "number of concurrent threads")
	flag.Parse()
	if n := flag.NArg(); n != 2 {
		// fmt.Printf("Usage: mirror (source dir) (destination dir), expected 2 arguments, got %d, %v\n", n, flag.Args())
		flag.Usage()
		os.Exit(1)
	}
	cfg.Source = flag.Arg(0)
	cfg.Destination = flag.Arg(1)
	if !isDir(cfg.Source) || !isDir(cfg.Destination) {
		fmt.Println("Both (source dir) and (destination dir) must be existing directories")
		os.Exit(1)
	}
	return cfg, parallel
}

func isDir(path string) bool {
	inf, err := os.Stat(path)
	if err != nil {
		fmt.Printf("Error accessing %s: %s\n", path, err)
		return false
	}
	return inf.IsDir()
}
