package lsa

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	TPing byte = iota
	TWrite
	TDelete
	TBig
	TBigFinish
	TBigCancel
)

type Revent struct {
	Typ     uint8
	Dir     string
	Name    string
	Stat    *Stat
	Content []byte
}

func (s *Revent) marshalLengthy(b *bytes.Buffer, lengthy []byte) (err error) {
	if err = binary.Write(b, binary.LittleEndian, uint32(len(lengthy))); err != nil {
		return
	}
	var wrote int
	if wrote, err = b.Write(lengthy); err != nil || wrote != len(lengthy) {
		if err == nil {
			err = fmt.Errorf("wrote %v instead of %v", wrote, len(lengthy))
		}
		return
	}
	return
}

func (s *Revent) Marshal(buf *[]byte) (err error) {
	b := bytes.NewBuffer(*buf)
	if err = binary.Write(b, binary.LittleEndian, s.Typ); err != nil {
		return
	}

	if s.Typ == TPing {
		*buf = b.Bytes()
		return
	}

	if err = s.marshalLengthy(b, []byte(s.Dir)); err != nil {
		return
	}

	if err = s.marshalLengthy(b, []byte(s.Name)); err != nil {
		return
	}

	if s.Typ == TWrite || s.Typ == TBig || s.Typ == TBigFinish {
		statBuf := make([]byte, 0, 20)
		if err = s.Stat.Marshal(&statBuf); err != nil {
			return
		}
		if err = s.marshalLengthy(b, statBuf); err != nil {
			return
		}

		if err = s.marshalLengthy(b, s.Content); err != nil {
			return
		}
	}

	*buf = b.Bytes()
	return
}

func UnmarshalLengthy(b io.Reader) (buf []byte, err error) {
	var u32 uint32
	if err = binary.Read(b, binary.LittleEndian, &u32); err != nil {
		return
	}
	buf = make([]byte, u32)
	_, err = io.ReadAtLeast(b, buf, int(u32))
	return
}

func UnmarshalRevent(b io.Reader) (s *Revent, err error) {
	s = new(Revent)
	if err = binary.Read(b, binary.LittleEndian, &s.Typ); err != nil {
		return nil, fmt.Errorf("UnmarshalRevent err:%v", err)
	}
	if s.Typ == TPing {
		return
	}

	var buf []byte
	if buf, err = UnmarshalLengthy(b); err != nil {
		return nil, fmt.Errorf("UnmarshalLengthy dir:%v ev:%s", err, s)
	}
	s.Dir = string(buf)


	if buf, err = UnmarshalLengthy(b); err != nil {
		return nil, fmt.Errorf("UnmarshalLengthy name:%v ev:%s", err, s)
	}
	s.Name = string(buf)

	if s.Typ == TWrite || s.Typ == TBig || s.Typ == TBigFinish {
		if buf, err = UnmarshalLengthy(b); err != nil {
			return nil, fmt.Errorf("UnmarshalLengthy stat:%v ev:%s", err, s)
		}
		if s.Stat, err = UnmarshalStat(buf); err != nil {
			return
		}

		if s.Content, err = UnmarshalLengthy(b); err != nil {
			return nil, fmt.Errorf("UnmarshalLengthy content:%v ev:%s", err, s)
		}
	}

	return
}

func (s *Revent) String() string {
	if s.Typ == TPing {
		return "ping"
	} else if s.Typ == TWrite {
		return fmt.Sprintf("write %s/%s %s content:%d", s.Dir, s.Name, s.Stat, len(s.Content))
	} else if s.Typ == TBig {
		return fmt.Sprintf("big %s/%s %s content:%d", s.Dir, s.Name, s.Stat, len(s.Content))
	} else if s.Typ == TBigFinish {
		return fmt.Sprintf("bigfin %s/%s %s content:%d", s.Dir, s.Name, s.Stat, len(s.Content))
	} else if s.Typ == TDelete {
		return fmt.Sprintf("delete %s/%s", s.Dir, s.Name)
	} else if s.Typ == TBigCancel {
		return fmt.Sprintf("cancel %s/%s", s.Dir, s.Name)
	} else {
		return "revent:unknown typ"
	}
}
