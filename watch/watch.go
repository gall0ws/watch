package main

import (
	"github.com/gall0ws/watch"

	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

const (
	shell    = "/bin/sh"
	shellArg = "-c"
)

var (
	argFile   bool
	pattern   string
	recursive bool
	dir       string
)

type Pipe struct {
	writeFn func([]byte) (int, error)
}

func (p *Pipe) Write(buf []byte) (int, error) {
	return p.writeFn(buf)
}

func runCommand(c chan<- *os.ProcessState, cmd *exec.Cmd, dst io.Writer) {
	defer func() { c <- cmd.ProcessState }()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(dst, err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintln(dst, err)
		return
	}
	ch := make(chan error, 2)
	copy := func(src io.Reader) {
		_, err := io.Copy(dst, src)
		ch <- err
	}
	go copy(stdout)
	go copy(stderr)

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(dst, err)
		return
	}
	for i := 0; i < cap(ch); i++ {
		if err := <-ch; err != nil {
			fmt.Fprintln(dst, err)
		}
	}
	cmd.Wait()
}

func init() {
	log.SetFlags(0)
	log.SetPrefix("watch: ")

	flag.BoolVar(&argFile, "n", false, "the name of the changed file is passed as last argument to cmd")
	flag.BoolVar(&recursive, "r", false, "watch recursively to subdirectories")
	flag.StringVar(&dir, "d", "", "directory to watch (default \"$PWD\")")
	flag.StringVar(&pattern, "p", "", "watch only for files matching the given pattern")
	flag.Usage = func() {
		fmt.Printf("Usage: %s [-nr] [-p regexp] [-d dir] cmd [cmd_args...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
}

func main() {
	var filter *regexp.Regexp
	var err error

	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalln("could not get current working directory:", err)
		}
		dir = cwd
	}
	if pattern != "" {
		filter, err = regexp.Compile(pattern)
		if err != nil {
			log.Fatalln(err)
		}
	}
	cmd := flag.Args()
	if len(cmd) == 0 {
		flag.Usage()
		return
	}
	cmdStr := strings.Join(cmd, " ")
	watchChan, err := watch.Watch(dir, recursive)
	if err != nil {
		log.Fatalln(err)
	}
	cmdChan := make(chan *os.ProcessState)
	cmdRunning := false
	runCmd := func(file string) {
		cmd := cmdStr
		if argFile {
			cmd += " " + path.Join(dir, file)
		}
		c := exec.Command(shell, shellArg, cmd)
		cmdRunning = true
		go runCommand(cmdChan, c, os.Stdout)
	}
	for {
		select {
		case file := <-watchChan:
			if cmdRunning || filter != nil && !filter.MatchString(file) {
				continue
			}
			runCmd(file)

		case pState := <-cmdChan:
			if !pState.Success() {
				fmt.Fprintf(os.Stdout, "%s: %v\n", cmdStr, pState)
			}
			cmdRunning = false
		}
	}
}
