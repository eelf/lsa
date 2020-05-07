package lsa

import (
	"fmt"
	"testing"
	"time"
)

func TestStatMarshalUnmarshal(t *testing.T) {
	tt, err := time.Parse(time.RFC3339, "2020-05-04T04:15:15+03:00")
	if err != nil {
		t.Fatalf("could not parse time: %v", err)
	}
	us := []*Stat{
		{true, true, 0777, 0xdeadbeef0, 0xcafe55feed},
		{false, false, 0755, tt.Unix(), 9240},
	}
	test := func(t *testing.T, u *Stat) {
		buf := make([]byte, 0, 8)
		err := u.Marshal(&buf)
		if err != nil {
			t.Fatalf("want noerror have %v", err)
		}

		v, err := UnmarshalStat(buf)
		if err != nil {
			t.Fatalf("want noerror have %v", err)
		}
		if v.isLink != u.isLink {
			t.Fatalf("isLink mismatch %v and %v", v.isLink, u.isLink)
		}
		if v.isDir != u.isDir {
			t.Fatalf("isDir mismatch %v and %v", v.isDir, u.isDir)
		}
		if v.mode != u.mode {
			t.Fatalf("mode mismatch %v and %v", v.mode, u.mode)
		}
		if v.mtime != u.mtime {
			t.Fatalf("mtime mismatch %v and %v", v.mtime, u.mtime)
		}
		if v.size != u.size {
			t.Fatalf("size mismatch %v and %v", v.size, u.size)
		}
	}

	for _, u := range us {
		t.Run(fmt.Sprint(u), func(t *testing.T){test(t, u)})
	}
}
