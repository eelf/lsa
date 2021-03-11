package main

import (
	"eelf.ru/lsa"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var pref string
var repo *Repository
var eventLog EventLog

func diff(dir string) error {
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		parent := path.Dir(dir)
		base := path.Base(dir)

		repo.DelFile(parent, base)

		eventLog.Add([]Event{{dir: parent, name: base, isDelete: true}})

		return nil
	}

	repo.AddDirIfNew(dir)
	repoInfo := repo.GetDirStat(dir)

	delDetection := make(map[string]bool)
	for name := range repoInfo {
		delDetection[name] = true
	}

	var events []Event

	for _, fi := range fis {
		delete(delDetection, fi.Name())
		el, ok := repoInfo[fi.Name()]

		newEl := lsa.NewStat(fi)
		if !ok || el.Diff(newEl) {

			if newEl.IsDir() {
				log.Printf("dir appeared or changed %v -> %v", el, newEl)
			}

			repoInfo[fi.Name()] = newEl
			events = append(events, Event{dir: dir, name: fi.Name()})

			//special case: now it is dir but earlier it hasn't existed or wasn't a dir
			if fi.IsDir() && (!ok || !el.IsDir()) {
				err := itDir(
					filepath.Join(dir, fi.Name()),
					func(dir2 string, fi2 os.FileInfo) {

						if fi2.IsDir() {
							log.Printf("dirrecu after parent appeared or changed %v", lsa.NewStat(fi2))
						}

						repo.AddFileToDir(dir2, fi2.Name(), lsa.NewStat(fi2))
						events = append(events, Event{dir: dir2, name: fi2.Name()})
					},
					func(dir2 string) {
						repo.AddDirIfNew(dir2)
					})
				if err != nil {
					return err
				}
			}
		}
	}
	for name := range delDetection {
		delete(repoInfo, name)
		events = append(events, Event{dir: dir, name: name, isDelete: true})
	}
	repo.SetDirStat(dir, repoInfo)
	if len(events) > 0 {
		eventLog.Add(events)
	}
	return nil
}

func itDir(dir string, fileCb func(string, os.FileInfo), dirCb func(string)) error {
	stack := []string{dir}
	for len(stack) > 0 {
		curDir := stack[len(stack)-1]
		stack = stack[0 : len(stack)-1]
		fis, err := ioutil.ReadDir(curDir)
		if err != nil {
			return err
		}
		dirCb(curDir)
		for _, fi := range fis {
			fileCb(curDir, fi)
			if fi.IsDir() {
				stack = append(stack, filepath.Join(curDir, fi.Name()))
			}
		}
	}
	return nil
}

func sshOptions() []string {
	options := []string{
		"-o", fmt.Sprint("ConnectTimeout=", 10),
		"-o", "LogLevel=ERROR",
		"-o", fmt.Sprint("ServerAliveInterval=", 3),
		"-o", fmt.Sprint("ServerAliveCountMax=", 4),
		//If set to yes, passphrase/password querying will be disabled. This option is useful in scripts and other batch jobs where no user is present to supply the password
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
	}
	if false {
		options = append(options, "-o", "Compression=yes")
	}

	return options
}

var Version string

func main() {
	log.SetPrefix(fmt.Sprintf("% -10s", ""))

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Println(Version, "mem sys", fmtSize(int(m.Sys)), "alloc", fmtSize(int(m.Alloc)))

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	flag.Parse()
	args := flag.Args()
	if len(args) < 2 {
		log.Fatalln("args needed")
	}

	if err := os.Chdir(args[0]); err != nil {
		log.Fatalln("cannot chdir", args[0], err)
	}

	var err error
	if pref, err = os.Getwd(); err != nil {
		log.Fatalln("cannot get wd", err)
	}

	repo = NewRepository()

	eventLog = NewEventLog()

	ch := make(chan string, 10000)
	Watch(pref, ch)

	err = itDir(
		".",
		func(dir string, fi os.FileInfo) {
			repo.AddFileToDir(dir, fi.Name(), lsa.NewStat(fi))
		},
		func(dir string) {
			repo.AddDirIfNew(dir)
		})
	if err != nil {
		log.Fatalln(err)
	}

	for _, s := range args[1:] {
		sp, err := NewSpace(s)
		if err != nil {
			log.Fatalln(err)
		}
		go sp.sender()
	}

	duration := 400 * time.Millisecond
	t := time.NewTimer(duration)
	t.Stop()
	batch := make(map[string]int)
	orderedBatch := make([]string, 0)

	runtime.ReadMemStats(&m)
	log.Println("repo ready. processing fs events sys:", fmtSize(int(m.Sys)), "alloc:", fmtSize(int(m.Alloc)))

	pathSeparator := fmt.Sprintf("%c", os.PathSeparator)
	order := 0
	for {
		select {
		case <-t.C:
			order = 0
			if cap(orderedBatch) < len(batch) {
				orderedBatch = make([]string, len(batch))
			}
			orderedBatch = orderedBatch[0:len(batch)]
			for dir, order := range batch {
				orderedBatch[order] = dir
			}
			for _, dir := range orderedBatch {
				err = diff(dir)
				if err != nil {
					log.Fatalln("diff err:", err)
				}
				delete(batch, dir)
			}
		case p := <-ch:
			dir := strings.Trim(strings.TrimPrefix(p, pref), pathSeparator)
			if len(dir) == 0 {
				dir = "."
			}
			if _, ok := batch[dir]; !ok {
				batch[dir] = order
				order++
			}
			t.Reset(duration)
		}
	}
}
