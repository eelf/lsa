package main

import (
	"context"
	"eelf.ru/lsa"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type Space struct {
	host, dir, user, sudo string
	speedBytes uint
	speedTime time.Duration
}

const bigSize = 2 << 20

func NewSpace(arg string) (s Space, err error) {
	parts := strings.Split(arg, ":")
	if len(parts) != 2 {
		err = fmt.Errorf("bad host:dir spec: %s", arg)
		return
	}
	s.host = parts[0]
	s.dir = parts[1]
	if hostUserParts := strings.Split(parts[0], "@"); len(hostUserParts) == 2 {
		s.host = hostUserParts[1]
		s.user = hostUserParts[0]
	}
	return
}

func execCommand(name string, arg ...string) *exec.Cmd {
	log.Println(name, arg)
	return exec.Command(name, arg...)
}

func fmtSize(size int) string {
	if size < 1<<10 {
		return fmt.Sprintf("%d B", size)
	} else if size < 1<<20 {
		return fmt.Sprintf("%d KiB", size>>10)
	} else {
		return fmt.Sprintf("%d MiB", size>>20)
	}
}

type bigFile struct {
	*os.File
	*lsa.Stat
}

func (s Space) senderOne() error {
	eventLog.AddClient(s.host)
	defer eventLog.RemoveClient(s.host)
	hostUser := s.host
	if len(s.user) > 0 {
		hostUser = s.user + "@" + hostUser
	}

	args := []string{"-e", "ssh " + strings.Join(sshOptions(), " ")}
	args = append(args, "-a", "--delete", "--stats", "./", hostUser+":"+s.dir+"/")

	command := execCommand("rsync", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rsync err:%s %s", err, string(output))
	}
	re := regexp.MustCompile("Number of files: (\\d+)\\s+Number of files transferred: (\\d+)\\s+Total file size: (\\d+) bytes\\s+Total transferred file size: (\\d+) bytes")
	rsyncStat := re.FindStringSubmatch(string(output))
	if len(rsyncStat) == 5 {
		log.Printf("rsync transferred %s(%s bytes) of %s(%s bytes)", rsyncStat[2], rsyncStat[4], rsyncStat[1], rsyncStat[3])
	} else {
		log.Println("bad rsync stat", string(output))
	}

	args = sshOptions()
	args = append(args, hostUser, "lsa-space", s.dir)
	command = execCommand("ssh", args...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}

	readOk := true
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		b := make([]byte, 512)
		for {
			_, err := stdout.Read(b)
			if err != nil {
				readOk = false
				cancel()
				break
			}
		}
	}()

	stdin, err := command.StdinPipe()
	if err != nil {
		return err
	}

	command.Stderr = os.Stderr

	if err = command.Start(); err != nil {
		return err
	}

	bigFiles := make(map[string]bigFile)
	defer func() {
		for _, bf := range bigFiles {
			bf.File.Close()
		}
	}()
	buf := make([]byte, 0, 8192)
	bigBuf := make([]byte, bigSize)
	state := ""
	prevState := state
	var timeout time.Duration
	for readOk {
		timeout = 15 * time.Second
		if len(bigFiles) != 0 || prevState != "all synced" {
			// do a empty cycle faster for printing "all synced" earlier
			timeout = 0
		}
		ctx, _ := context.WithTimeout(ctx, timeout)
		evs := eventLog.Get(s.host, ctx)
		if !readOk {
			break
		}
		if prevState != state {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			log.Println(s.host, state, "mem sys", fmtSize(int(m.Sys)), "alloc", fmtSize(int(m.Alloc)), fmtSize(int(s.speedBytes)) + "ps")
			prevState = state
		}

		if len(bigFiles) != 0 {
			state = "sending big"
			for path, bf := range bigFiles {
				dir, name := filepath.Split(path)
				rEv := lsa.Revent{Typ: lsa.TBig, Dir: strings.TrimRight(dir, "/"), Name: name, Stat: bf.Stat}
				curOff, err := bf.File.Seek(0, io.SeekCurrent)
				if err != nil {
					return err
				}
				size := int(bf.Stat.Size() - curOff)
				if size > bigSize {
					size = bigSize
				}
				n, err := io.ReadAtLeast(bf.File, bigBuf, size)
				if err != nil {
					return err
				}
				if n != size {
					return fmt.Errorf("big.ReadAtLeast %d %d", n, size)
				}
				rEv.Content = bigBuf[:n]

				if curOff+int64(n) == bf.Stat.Size() {
					bf.File.Close()
					delete(bigFiles, path)
					rEv.Typ = lsa.TBigFinish
				}
				if err = s.write(stdin, buf, &rEv); err != nil {
					return err
				}

				break
			}
		}
		if len(evs) == 0 && len(bigFiles) == 0 {
			state = "all synced"
			rEv := lsa.Revent{Typ: lsa.TPing}
			if err = s.write(stdin, buf, &rEv); err != nil {
				return err
			}
			continue
		}
		for _, ev := range evs {
			state = "syncing"
			path := filepath.Join(ev.dir, ev.name)

			if bf, ok := bigFiles[path]; ok {
				rEv := lsa.Revent{Typ: lsa.TBigCancel, Dir: ev.dir, Name: ev.name}
				if err = s.write(stdin, buf, &rEv); err != nil {
					return err
				}
				bf.File.Close()
				delete(bigFiles, path)
			}

			rEv := lsa.Revent{Dir: ev.dir, Name: ev.name}
			fp, err := os.Open(path)
			if err != nil {
				if !os.IsNotExist(err) {
					return err
				}
				if !ev.isDelete {
					continue
				}
				rEv.Typ = lsa.TDelete
			} else {
				fi, err := fp.Stat()
				if err != nil {
					fp.Close()
					return err
				}
				rEv.Stat = lsa.NewStat(fi)
				if fi.IsDir() {
					rEv.Typ = lsa.TWrite
					fp.Close()
				} else if fi.Size() > bigSize {
					rEv.Typ = lsa.TBig
					bigFiles[path] = bigFile{
						File: fp,
						Stat: lsa.NewStat(fi),
					}
					_, err = io.ReadAtLeast(fp, bigBuf, bigSize)
					if err != nil {
						return err
					}
					rEv.Content = bigBuf
				} else {
					rEv.Typ = lsa.TWrite
					rEv.Content, err = ioutil.ReadAll(fp)
					fp.Close()
					if err != nil {
						return err
					}
				}
			}

			if err = s.write(stdin, buf, &rEv); err != nil {
				return err
			}
		}
	}
	if err = command.Process.Kill(); err != nil {
		return err
	}
	return fmt.Errorf("read is not ok")
}

func (s *Space) write(stdin io.WriteCloser, buf []byte, rEv *lsa.Revent) (err error) {
	var wrote int

	buf = buf[:0]
	if err = rEv.Marshal(&buf); err != nil {
		return
	}
	w := stdin
	t := time.Now()
	if wrote, err = w.Write(buf); err != nil || wrote != len(buf) {
		return
	}
	s.speedBytes += uint(wrote)
	s.speedTime += time.Now().Sub(t)
	s.speedBytes = uint(float64(s.speedBytes) / s.speedTime.Seconds())
	s.speedTime = time.Second

	return
}

func (s Space) sender() {
	for {
		err := s.senderOne()
		log.Println(s.host, "sender error:", err)
		time.Sleep(5 * time.Second)
	}
}
