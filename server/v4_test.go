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

func TestSameRadioTruncatedCopyIsDuplicate(t *testing.T) {
	// a site that heard only the start of a transmission, then the full copy from another site
	feedSrc := func(rs []rec, same bool) func(func(uint, int, string, bool) bool) {
		return func(y func(uint, int, string, bool) bool) {
			for _, r := range rs {
				if !y(r.id, r.size, r.atype, same) {
					return
				}
			}
		}
	}
	a, ids := decideDuplicateSrc(200000, "audio/mpeg", feedSrc([]rec{{1, 20000, "audio/mpeg"}}, true))
	if a != DuplicateReplace || len(ids) != 1 {
		t.Fatal(a, ids)
	}
	if a, _ := decideDuplicateSrc(20000, "audio/mpeg", feedSrc([]rec{{1, 200000, "audio/mpeg"}}, true)); a != DuplicateReject {
		t.Fatal(a)
	}
	// a different radio keeps the old behaviour
	if a, _ := decideDuplicateSrc(200000, "audio/mpeg", feedSrc([]rec{{1, 20000, "audio/mpeg"}}, false)); a != DuplicateNone {
		t.Fatal(a)
	}
	if callSourceID(uint(4090280)) != 4090280 || callSourceID(nil) != 0 {
		t.Fatal("callSourceID")
	}
}
