package main

import "testing"

type rec struct {
	id    uint
	size  int
	atype string
}

func feed(rs []rec) func(func(uint, int, string) bool) {
	return func(y func(uint, int, string) bool) {
		for _, r := range rs {
			if !y(r.id, r.size, r.atype) {
				return
			}
		}
	}
}

func TestLongerCopyReplacesShorter(t *testing.T) {
	a, ids := decideDuplicate(100000, "audio/mpeg", feed([]rec{{1, 80000, "audio/mpeg"}}))
	if a != DuplicateReplace || len(ids) != 1 || ids[0] != 1 {
		t.Fatal(a, ids)
	}
}

func TestShorterCopyRejected(t *testing.T) {
	if a, _ := decideDuplicate(80000, "audio/mpeg", feed([]rec{{1, 100000, "audio/mpeg"}})); a != DuplicateReject {
		t.Fatal(a)
	}
}

func TestDifferentFormatsKept(t *testing.T) {
	if a, _ := decideDuplicate(100000, "audio/mp4", feed([]rec{{1, 100000, "audio/mpeg"}})); a != DuplicateNone {
		t.Fatal(a)
	}
}

func TestShortKeyUpDoesNotBlockConversation(t *testing.T) {
	if a, _ := decideDuplicate(200000, "audio/mpeg", feed([]rec{{1, 5000, "audio/mpeg"}})); a != DuplicateNone {
		t.Fatal(a)
	}
}

func TestPhantomOnlyUnnamed(t *testing.T) {
	if !talkgroupIsPhantom(&Talkgroup{Id: 12582922, Label: "12582922", Name: "12582922"}) {
		t.Fatal("unnamed big id should be phantom")
	}
	if !talkgroupIsPhantom(&Talkgroup{Id: 27, Label: "27"}) {
		t.Fatal("unnamed small id should be phantom")
	}
	if talkgroupIsPhantom(&Talkgroup{Id: 27, Label: "JAC Fire", Name: "Jackson Fire"}) {
		t.Fatal("named talkgroup must never be pruned")
	}
	if talkgroupIsPhantom(&Talkgroup{Id: 4501, Label: "4501"}) {
		t.Fatal("normal-range id is not phantom")
	}
	if !talkgroupIsUnnamed(&Talkgroup{Id: 4501, Label: "4501"}) {
		t.Fatal("4501 is unnamed")
	}
}
