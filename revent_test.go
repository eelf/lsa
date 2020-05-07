package lsa

import (
	"bytes"
	"testing"
)


func TestReventMarshalUnmarshal(t *testing.T) {
	us := &Stat{true, true, 0777, 0xdeadbeef0, 0xcafe55feed}

	rEv := Revent{Typ: TWrite, Dir: "dira", Stat: us}
	var err error

	buf := make([]byte, 0, 8192)
	if err = rEv.Marshal(&buf); err != nil {
		t.Fatal(err)
	}

	var sEv *Revent
	if sEv, err = UnmarshalRevent(bytes.NewReader(buf)); err != nil {
		t.Fatal(err)
	}
	if sEv.Typ != rEv.Typ {
		t.Fatal("typ")
	}
	if sEv.Dir != rEv.Dir {
		t.Fatal("dir")
	}
	if sEv.Name != rEv.Name {
		t.Fatal("name")
	}
	if *sEv.Stat != *rEv.Stat {
		t.Fatal("stat")
	}
}
