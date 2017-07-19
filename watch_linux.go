package watch

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

func Watch(dirpath string, recursive bool) (<-chan string, error) {
	di, err := os.Stat(dirpath)
	if err != nil {
		return nil, err
	}
	if !di.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dirpath)
	}
	fd, err := syscall.InotifyInit()
	if err != nil {
		return nil, err
	}
	addWatch := func(path string, info os.FileInfo, unused error) error {
		if !info.IsDir() {
			return nil
		}
		_, err = syscall.InotifyAddWatch(fd, path, syscall.IN_CLOSE_WRITE)
		return err
	}
	if recursive {
		err = filepath.Walk(dirpath, addWatch)
	} else {
		err = addWatch(dirpath, di, nil)
	}
	if err != nil {
		return nil, err
	}
	c := make(chan string)
	go func() {
		buf := [syscall.SizeofInotifyEvent + syscall.PathMax]byte{}
		for {
			syscall.Read(fd, buf[0:])
			ev := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[0]))
			if ev.Len <= 0 {
				log.Println("warning: event ignored: ev.Len:", ev.Len)
				continue
			}
			cpath := (*[syscall.PathMax]byte)(unsafe.Pointer(&buf[syscall.SizeofInotifyEvent]))
			c <- string(bytes.TrimRight(cpath[:], "\x00"))
		}
	}()
	return c, nil
}
