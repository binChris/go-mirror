package mirror

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/binChris/mirror/config"
)

type Frontend interface {
	Progress(msg string)
	Fatal(msg string)
	Choice(msg string, options string) rune
}

type mirror struct {
	frontend       Frontend
	m              sync.Mutex
	queue          []config.Config
	throttle       chan struct{}
	wg             sync.WaitGroup
	dirsCreated    uint64
	dirsDeleted    uint64
	filesCopied    uint64
	filesDeleted   uint64
	filesIdentical uint64
}

// Run will start the mirroring process with 'parallel' processes and return when done
func Run(cfg config.Config, parallel int, frontend Frontend) {
	if parallel < 1 {
		parallel = 1
	}
	m := mirror{
		frontend: frontend,
		queue:    make([]config.Config, 0, 100),
		throttle: make(chan struct{}, parallel),
	}
	m.add([]config.Config{cfg})
	for {
		cfg, ok := m.get()
		if !ok {
			break
		}
		m.process(cfg)
	}
	m.wg.Wait()
	fmt.Printf("%d/%d dirs created/deleted, %d/%d files copied/deleted, %d files identical\n",
		m.dirsCreated, m.dirsDeleted,
		m.filesCopied, m.filesDeleted,
		m.filesIdentical,
	)
}

func (m *mirror) add(cfgs []config.Config) {
	m.m.Lock()
	defer m.m.Unlock()
	m.queue = append(m.queue, cfgs...)
}

func (m *mirror) get() (config.Config, bool) {
	m.m.Lock()
	defer m.m.Unlock()
	if len(m.queue) == 0 {
		return config.Config{}, false
	}
	cfg := m.queue[0]
	m.queue = m.queue[1:]
	return cfg, true
}

func (m *mirror) process(cfg config.Config) {
	m.throttle <- struct{}{}
	defer func() { <-m.throttle }()
	m.frontend.Progress(fmt.Sprintf("Mirroring %s to %s", cfg.Source, cfg.Destination))
	subs, delDirs, delFiles, cpFiles := m.compareSourceWithDestination(cfg)
	m.add(subs)
	for _, d := range delDirs {
		m.wg.Add(1)
		go func(d string) {
			defer m.wg.Done()
			// delete as soon as possible, don't throttle
			d = filepath.Join(cfg.Destination, d)
			if err := os.RemoveAll(d); err != nil {
				m.frontend.Fatal(fmt.Sprintf("Cannot delete dir '%s': %s", d, err))
			}
			atomic.AddUint64(&m.dirsDeleted, 1)
		}(d)
	}
	for _, f := range delFiles {
		m.wg.Add(1)
		go func(f string) {
			defer m.wg.Done()
			// delete as soon as possible, don't throttle
			f = filepath.Join(cfg.Destination, f)
			if err := os.Remove(f); err != nil {
				m.frontend.Fatal(fmt.Sprintf("Cannot delete file '%s': %s", f, err))
			}
			atomic.AddUint64(&m.filesDeleted, 1)
		}(f)
	}
	for _, cp := range cpFiles {
		m.wg.Add(1)
		go func(cp string) {
			defer m.wg.Done()
			// throttle copying files
			m.throttle <- struct{}{}
			defer func() { <-m.throttle }()
			s := filepath.Join(cfg.Source, cp)
			d := filepath.Join(cfg.Destination, cp)
			m.frontend.Progress(fmt.Sprintf("Copy %s to %s\n", s, d))
			if err := copyFile(s, d); err != nil {
				m.frontend.Fatal(err.Error())
			}
			atomic.AddUint64(&m.filesCopied, 1)
		}(cp)
	}
}

func (m *mirror) compareSourceWithDestination(cfg config.Config) (subs []config.Config, delDirs, delFiles, cpFiles []string) {
	sDirs, sFiles, err := readDir(cfg.Source, false)
	if err != nil {
		m.frontend.Fatal(fmt.Sprintf("Cannot read directory '%s': %s", cfg.Source, err))
	}
	dDirs, dFiles, err := readDir(cfg.Destination, true)
	if err != nil {
		m.frontend.Fatal(fmt.Sprintf("Cannot read directory '%s': %s", cfg.Destination, err))
	}
	subs = make([]config.Config, 0)
	delDirs = make([]string, 0)
	delFiles = make([]string, 0)
	cpFiles = make([]string, 0)
	// determine source subs
	for dirName, inf := range sDirs {
		dDir := filepath.Join(cfg.Destination, dirName)
		if _, exInDst := dDirs[dirName]; !exInDst {
			if !m.allow(cfg.CreateDir, "Create dir '%s'", dDir) {
				continue
			}
			m.frontend.Progress(fmt.Sprintf("Creating dir %s", dDir))
			os.Mkdir(dDir, inf.Type().Perm())
			atomic.AddUint64(&m.dirsCreated, 1)
		}
		subCfg := cfg
		subCfg.Source = filepath.Join(cfg.Source, dirName)
		subCfg.Destination = filepath.Join(cfg.Destination, dirName)
		subs = append(subs, subCfg)
	}
	// determine destination dirs to be deleted
	for dst := range dDirs {
		if _, exInSrc := sDirs[dst]; !exInSrc {
			if !m.allow(cfg.DeleteDir, "Delete dir '%s'", dst) {
				continue
			}
			delDirs = append(delDirs, dst)
		}
	}
	// determine destination files to be deleted
	for dst := range dFiles {
		if _, exInSrc := sFiles[dst]; !exInSrc {
			if !m.allow(cfg.DeleteFile, "Delete file '%s'", dst) {
				continue
			}
			delFiles = append(delFiles, dst)
		}
	}
	// determine files to be copied
	for fName := range sFiles {
		sPath := filepath.Join(cfg.Source, fName)
		dPath := filepath.Join(cfg.Destination, fName)
		if _, exInDst := dFiles[fName]; !exInDst {
			if !m.allow(cfg.CreateFile, "Create file '%s'", dPath) {
				continue
			}
			cpFiles = append(cpFiles, fName)
		} else if m.filesAreDifferent(sPath, dPath) {
			if !m.allow(cfg.OverwriteFile, "Overwrite file '%s'", dPath) {
				continue
			}
		} else {
			atomic.AddUint64(&m.filesIdentical, 1)
		}
	}
	return subs, delDirs, delFiles, cpFiles
}

func (m *mirror) filesAreDifferent(path1, path2 string) bool {
	fi1, err := os.Stat(path1)
	if err != nil {
		m.frontend.Fatal(fmt.Sprintf("Cannot get file info for '%s': %s", path1, err))
	}
	fi2, err := os.Stat(path2)
	if err != nil {
		m.frontend.Fatal(fmt.Sprintf("Cannot get file info for '%s': %s", path2, err))
	}
	return fi1.Size() != fi2.Size() || fi1.ModTime().Sub(fi2.ModTime()) > time.Second
}

func (m *mirror) allow(flagPtr *rune, msg string, msgVals ...interface{}) bool {
	m.m.Lock()
	defer m.m.Unlock()
	if *flagPtr == 'a' {
		return true
	}
	if *flagPtr == 'x' {
		return false
	}
	switch m.frontend.Choice(fmt.Sprintf(msg+" (y=yes,n=no,a=all,x=none,q=quit)", msgVals...), "ynaq") {
	case 'y':
		return true
	case 'n':
		return false
	case 'a':
		*flagPtr = 'a'
		return true
	case 'x':
		*flagPtr = 'x'
		return false
	case 'q':
		os.Exit(1)
	}
	panic("choice")
}

func readDir(path string, create bool) (dirs map[string]fs.DirEntry, files map[string]fs.DirEntry, err error) {
	ee, err := os.ReadDir(path)
	if err != nil {
		return nil, nil, err
	}
	dirs = make(map[string]fs.DirEntry)
	files = make(map[string]fs.DirEntry)
	for _, e := range ee {
		if e.IsDir() {
			dirs[e.Name()] = e
		} else {
			files[e.Name()] = e
		}
	}
	return dirs, files, nil
}

func copyFile(src, dst string) error {
	copy := func() error {
		srcF, err := os.Open(src)
		if err != nil {
			return fmt.Errorf("Could not open '%s' for reading", src)
		}
		defer srcF.Close()
		dstF, err := os.Create(dst)
		if err != nil {
			return fmt.Errorf("Could not create '%s' for writing", dst)
		}
		defer dstF.Close()
		if _, err := io.Copy(dstF, srcF); err != nil {
			return fmt.Errorf("error copying file '%s': %s", src, err)
		}
		return nil
	}
	if err := copy(); err != nil {
		return err
	}
	inf, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("get file info for '%s': %w", src, err)
	}
	if err := os.Chtimes(dst, inf.ModTime(), inf.ModTime()); err != nil {
		return fmt.Errorf("set modification time for '%s': %w", dst, err)
	}
	return nil
}
