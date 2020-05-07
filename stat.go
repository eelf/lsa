package lsa

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

type Stat struct {
	isDir  bool
	isLink bool
	mode   uint16
	mtime  int64
	size   int64
}

func NewStat(fi os.FileInfo) *Stat {
	return &Stat{
		fi.IsDir(),
		fi.Mode()&os.ModeSymlink == os.ModeSymlink,
		uint16(fi.Mode() & 0777),
		fi.ModTime().Unix(),
		fi.Size(),
	}
}

func (s *Stat) Mtime() int64 {
	return s.mtime
}

func (s *Stat) Mode() os.FileMode {
	return os.FileMode(s.mode)
}

func (s *Stat) IsDir() bool {
	return s.isDir
}

func (s *Stat) IsLink() bool {
	return s.isLink
}

func (s *Stat) Size() int64 {
	return s.size
}

func (s *Stat) Diff(o *Stat) bool {
	if s.isDir {
		return !o.isDir || s.mode != o.mode
	}
	if s.isLink {
		return !o.isLink || s.size != o.size
	}
	return o.isDir || o.isLink || s.mode != o.mode || s.size != o.size || s.mtime != o.mtime
}

func (s *Stat) Marshal(buf *[]byte) (err error) {
	b := bytes.NewBuffer(*buf)

	v := uint32(s.mode)
	if s.isDir {
		v |= 1 << 16
	}
	if s.isLink {
		v |= 1 << 17
	}
	if err = binary.Write(b, binary.LittleEndian, v); err != nil {
		return
	}

	if err = binary.Write(b, binary.LittleEndian, s.mtime); err != nil {
		return
	}

	if err = binary.Write(b, binary.LittleEndian, s.size); err != nil {
		return
	}
	*buf = b.Bytes()
	return
}

func UnmarshalStat(buf []byte) (s *Stat, err error) {
	s = new(Stat)
	b := bytes.NewReader(buf)

	var v uint32
	if err = binary.Read(b, binary.LittleEndian, &v); err != nil {
		return
	}
	s.mode = uint16(v & 0777)
	if v&(1<<16) != 0 {
		s.isDir = true
	}
	if v&(1<<17) != 0 {
		s.isLink = true
	}

	if err = binary.Read(b, binary.LittleEndian, &s.mtime); err != nil {
		return
	}

	if err = binary.Read(b, binary.LittleEndian, &s.size); err != nil {
		return
	}
	return
}

func (s *Stat) String() string {
	return fmt.Sprintf("d:%t l:%t m:%o mt:%s s:%d", s.isDir, s.isLink, s.mode, time.Unix(s.mtime, 0).Format(time.RFC3339), s.size)
}
