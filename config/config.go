package config

import (
	"flag"
	"fmt"
	"os"
)

type Config struct {
	Source        string
	Destination   string
	CreateDir     *rune
	DeleteDir     *rune
	CreateFile    *rune
	OverwriteFile *rune
	DeleteFile    *rune
}

func FromCommandLine() (Config, int) {
	var cfg Config
	parallel := 5
	force := false
	flag.BoolVar(&force, "force", force, "create/delete in destination without confirmation")
	flag.IntVar(&parallel, "parallel", parallel, "number of concurrent threads")
	flag.Parse()
	if n := flag.NArg(); n != 2 {
		usage()
		fmt.Printf("Expected 2 arguments, got %d, %v\n", n, flag.Args())
		os.Exit(1)
	}
	cfg.Source = flag.Arg(0)
	cfg.Destination = flag.Arg(1)
	cd, dd, cf, of, df := '-', '-', '-', '-', '-'
	if force {
		cd, dd, cf, of, df = 'a', 'a', 'a', 'a', 'a'
	}
	cfg.CreateDir = &cd
	cfg.DeleteDir = &dd
	cfg.CreateFile = &cf
	cfg.OverwriteFile = &of
	cfg.DeleteFile = &df
	if !isDir(cfg.Source) || !isDir(cfg.Destination) {
		fmt.Println("Both (source dir) and (destination dir) must be existing directories")
		os.Exit(1)
	}
	return cfg, parallel
}

func usage() {
	fmt.Println("Usage: mirror (source dir) (destination dir)")
	flag.PrintDefaults()
}

func isDir(path string) bool {
	inf, err := os.Stat(path)
	if err != nil {
		fmt.Printf("Error accessing %s: %s\n", path, err)
		return false
	}
	return inf.IsDir()
}
