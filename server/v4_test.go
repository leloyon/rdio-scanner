package main

import "testing"

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
