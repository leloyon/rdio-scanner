package main

import (
	"strings"
	"testing"
)

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
	feedSrc := func(rs []rec, same int) func(func(uint, int, string, int) bool) {
		return func(y func(uint, int, string, int) bool) {
			for _, r := range rs {
				if !y(r.id, r.size, r.atype, same) {
					return
				}
			}
		}
	}
	a, ids := decideDuplicateSrc(200000, "audio/mpeg", feedSrc([]rec{{1, 20000, "audio/mpeg"}}, 1))
	if a != DuplicateReplace || len(ids) != 1 {
		t.Fatal(a, ids)
	}
	if a, _ := decideDuplicateSrc(20000, "audio/mpeg", feedSrc([]rec{{1, 200000, "audio/mpeg"}}, 1)); a != DuplicateReject {
		t.Fatal(a)
	}
	// a different radio keeps the old behaviour
	if a, _ := decideDuplicateSrc(200000, "audio/mpeg", feedSrc([]rec{{1, 20000, "audio/mpeg"}}, 0)); a != DuplicateNone {
		t.Fatal(a)
	}
	// a different radio is a different transmission: keep both, whatever the sizes
	if a, _ := decideDuplicateSrc(200000, "audio/mpeg", feedSrc([]rec{{1, 200000, "audio/mpeg"}}, -1)); a != DuplicateNone {
		t.Fatal(a)
	}
	// equal stored sizes with the same radio keep one copy
	if a, _ := decideDuplicateSrc(96000, "audio/mpeg", feedSrc([]rec{{1, 96000, "audio/mpeg"}}, 1)); a != DuplicateReject {
		t.Fatal(a)
	}
	if callSourceID(uint(4090280)) != 4090280 || callSourceID(nil) != 0 {
		t.Fatal("callSourceID")
	}
}

func TestShortCopiesComparedByRealLength(t *testing.T) {
	// both stored as 3 s of mp3 (96000), but the new copy was really longer
	stored := 96000
	existing := scaleToStored(40000, 56000, stored, stored)
	if a, ids := decideDuplicateSrc(stored, "audio/mpeg", func(y func(uint, int, string, int) bool) { y(1, existing, "audio/mpeg", 1) }); a != DuplicateReplace || len(ids) != 1 {
		t.Fatal(a, ids)
	}
	// and the other way round the shorter new copy is rejected
	existing = scaleToStored(56000, 40000, stored, stored)
	if a, _ := decideDuplicateSrc(stored, "audio/mpeg", func(y func(uint, int, string, int) bool) { y(1, existing, "audio/mpeg", 1) }); a != DuplicateReject {
		t.Fatal(a)
	}
	// unknown real sizes fall back to the stored size
	if scaleToStored(0, 1, 2, 77) != 77 {
		t.Fatal("fallback")
	}
}

func TestDuplicateExplainsItself(t *testing.T) {
	feed1 := func(size int, src int) func(func(uint, int, string, int) bool) {
		return func(y func(uint, int, string, int) bool) { y(7, size, "audio/x-wav", src) }
	}
	a, _, why := decideDuplicateExplain(1000, "audio/x-wav", feed1(1000, 1))
	if a != DuplicateReject || len(why) != 1 || !strings.Contains(why[0], "rejected") || !strings.Contains(why[0], "1000 >= 1000") {
		t.Fatal(a, why)
	}
	a, _, why = decideDuplicateExplain(1000, "audio/x-wav", feed1(1000, -1))
	if a != DuplicateNone || !strings.Contains(why[0], "different radio") {
		t.Fatal(a, why)
	}
	a, _, why = decideDuplicateExplain(1000, "audio/mpeg", feed1(1000, 1))
	if a != DuplicateNone || !strings.Contains(why[0], "different format") {
		t.Fatal(a, why)
	}
	a, _, why = decideDuplicateExplain(1000, "audio/x-wav", feed1(300, 0))
	if a != DuplicateNone || !strings.Contains(why[0], "sizes too different") {
		t.Fatal(a, why)
	}
}
