package console

import (
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

type Console struct {
	waitForInput sync.Mutex
	nextProgress time.Time
}

var oldTermState *term.State

func init() {
	var err error
	oldTermState, err = term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Printf("Cannot switch to raw terminal mode: %s\n", err)
	}
}

func Cleanup() {
	if oldTermState != nil {
		term.Restore(int(os.Stdin.Fd()), oldTermState)
	}
}

func New() *Console {
	return &Console{
		waitForInput: sync.Mutex{},
		nextProgress: time.Now(),
	}
}

// Progress outputs max. 1 message per second. If waiting on input, output will be skipped
func (c *Console) Progress(msg string) {
	if c.nextProgress.After(time.Now()) {
		return
	}
	if !c.waitForInput.TryLock() {
		return
	}
	defer c.waitForInput.Unlock()
	c.nextProgress = time.Now().Add(time.Second)
	fmt.Println("...(", msg, ")")
}

func (c *Console) Fatal(msg string) {
	fmt.Println("\n", msg)
	os.Exit(1)
}

func (c *Console) Choice(msg string, options string) rune {
	c.waitForInput.Lock()
	defer c.waitForInput.Unlock()
	for {
		fmt.Print(msg, ": ")
		b := make([]byte, 1)
		_, _ = os.Stdin.Read(b)
		r := rune(b[0])
		for _, o := range options {
			if r == o {
				fmt.Println()
				return r
			}
		}
		fmt.Println("Invalid answer")
	}
}
