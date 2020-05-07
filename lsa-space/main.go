package main

import (
	"bufio"
	"eelf.ru/lsa"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

func writeContents(file string, stat *lsa.Stat, contents []byte) error {
	lstat, err := os.Lstat(file)

	if err == nil {
		// file already exists, if it is symlink or dir then it should be removed due to inability to make atomic rename
		if lstat.IsDir() != stat.IsDir() || lstat.Mode()&os.ModeSymlink == os.ModeSymlink {
			if err = os.RemoveAll(file); err != nil {
				return fmt.Errorf("cannot remove %s: %s", file, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot lstat %s: %s", file, err)
	}

	if stat.IsDir() {
		if err = os.Mkdir(file, 0777); err != nil {
			return fmt.Errorf("cannot mkdir %s: %s", file, err)
		}
		if err = os.Chmod(file, stat.Mode()); err != nil {
			return fmt.Errorf("cannot chmod dir %s: %s", file, err)
		}
	} else if stat.IsLink() {
		if err = os.Symlink(string(contents), file); err != nil {
			return fmt.Errorf("cannot create symlink %s: %s", file, err)
		}
	} else {
		if err = writeFile(file, stat, contents); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(file string, stat *lsa.Stat, contents []byte) error {
	fp, err := ioutil.TempFile(".", "lsa")
	if err != nil {
		return fmt.Errorf("failed to make temp file: %s", err)
	}
	tmpName := fp.Name()

	wrote, err := fp.Write(contents)
	fp.Close()
	if err != nil || wrote != len(contents) {
		return fmt.Errorf("cannot write (wrote %d instead of %d): %s", wrote, len(contents), err)
	}

	if err = finishFile(tmpName, file, stat); err != nil {
		return err
	}

	return nil
}

func finishFile(tmpName, file string, stat *lsa.Stat) error {
	var err error
	if err = os.Chmod(tmpName, stat.Mode()); err != nil {
		return fmt.Errorf("cannot chmod %s: %s", tmpName, err)
	}

	if err = os.Chtimes(tmpName, time.Unix(stat.Mtime(), 0), time.Unix(stat.Mtime(), 0)); err != nil {
		return fmt.Errorf("cannot chtimes %s: %s", tmpName, err)
	}

	if err = os.Rename(tmpName, file); err != nil {
		return fmt.Errorf("cannot rename %s to %s: %s", tmpName, file, err)
	}
	return nil
}

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	if len(hostname) > 9 {
		hostname = hostname[:10]
	}
	log.SetPrefix(fmt.Sprintf("% -10s", hostname))
	flag.Parse()
	args := flag.Args()

	if len(args) != 1 || len(args[0]) < 1 {
		log.Fatalln("not enough args", len(args), args)
	}
	fi, err := os.Stat(args[0])
	if err != nil && !os.IsNotExist(err) || err == nil && !fi.IsDir() {
		log.Fatalln("target exists as not a dir or could not stat (not because not exists)", args[0], err)
	}
	if err != nil && os.IsNotExist(err) {
		if err = os.MkdirAll(args[0], 0777); err != nil {
			log.Fatalln("could not mkdir", args[0], err)
		}
	}
	if err = os.Chdir(args[0]); err != nil {
		log.Fatalln("could not chdir", args[0], err)
	}

	duration := 60 * time.Second
	t := time.NewTimer(duration)
	t.Reset(duration)

	reCh := make(chan *lsa.Revent)
	go func() {
		b := bufio.NewReaderSize(os.Stdin, 2<<20)
		for {
			re, err := lsa.UnmarshalRevent(b)
			if err != nil {
				log.Fatalln(err)
			}
			reCh <- re
		}
	}()
	var re *lsa.Revent
	bigFiles := make(map[string]*os.File)

	pingReply := make([]byte, 1)
	rEv := lsa.Revent{Typ:lsa.TPing}
	if err := rEv.Marshal(&pingReply); err != nil {
		log.Fatalln(err)
	}

	for {
		select {
		case <-t.C:
			log.Fatalln("no events for", duration)
		case re = <-reCh:
			t.Reset(duration)
		}

		if re.Typ == lsa.TPing {
			wrote, err := os.Stdout.Write(pingReply)
			if err != nil || wrote != len(pingReply) {
				log.Fatalln(err)
			}
		} else if re.Typ == lsa.TWrite {
			err := writeContents(filepath.Join(re.Dir, re.Name), re.Stat, re.Content)
			if err != nil {
				log.Fatalln("write failed", err)
			}
		} else if re.Typ == lsa.TDelete {
			err := os.RemoveAll(filepath.Join(re.Dir, re.Name))
			if err != nil {
				log.Fatalln("delete failed", err)
			}
		} else if re.Typ == lsa.TBig || re.Typ == lsa.TBigFinish {
			path := filepath.Join(re.Dir, re.Name)
			fp, ok := bigFiles[path]
			if !ok {
				if re.Typ == lsa.TBigFinish {
					log.Fatalln("bigfinish no bigfile")
				}
				fp, err = ioutil.TempFile(".", "lsa")
				if err != nil {
					log.Fatalln(err)
				}
				bigFiles[path] = fp
			}
			wrote, err := fp.Write(re.Content)
			if err != nil || wrote != len(re.Content) {
				log.Fatalf("cannot write (wrote %d instead of %d): %s", wrote, len(re.Content), err)
			}
			if re.Typ == lsa.TBigFinish {
				delete(bigFiles, path)
				tmpName := fp.Name()
				fp.Close()
				if err = finishFile(tmpName, path, re.Stat); err != nil {
					log.Fatalln(err)
				}
			}
		} else if re.Typ == lsa.TBigCancel {
			path := filepath.Join(re.Dir, re.Name)
			fp, ok := bigFiles[path]
			if !ok {
				log.Fatalln("no bigfile", path)
			}
			err := os.Remove(fp.Name())
			if err != nil {
				log.Fatalln("bigcancel remove failed", err)
			}
			fp.Close()
			delete(bigFiles, path)
		}
	}
}
