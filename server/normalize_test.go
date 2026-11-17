package main

import "testing"

func TestEffectiveAudioMode(t *testing.T) {
	const (
		off   = AUDIO_CONVERSION_DISABLED
		plain = AUDIO_CONVERSION_ENABLED
		norm  = AUDIO_CONVERSION_ENABLED_NORM
		loud  = AUDIO_CONVERSION_ENABLED_LOUD_NORM
	)
	cases := []struct {
		name   string
		global uint
		list   string
		sys    uint
		tg     uint
		want   uint
	}{
		{"empty list keeps global", plain, "", 1, 101, plain},
		{"blank list keeps global", plain, "  \n ", 1, 101, plain},
		{"listed talkgroup normalized", plain, "1:101", 1, 101, norm},
		{"unlisted talkgroup unchanged", plain, "1:101", 1, 102, plain},
		{"other system unchanged", plain, "1:101", 2, 101, plain},
		{"loud suffix", plain, "1:101:loud", 1, 101, loud},
		{"loud suffix case-insensitive", plain, "1:101:LOUD", 1, 101, loud},
		{"bad suffix ignored", plain, "1:101:quiet", 1, 101, plain},
		{"disabled stays disabled", off, "1:101", 1, 101, off},
		{"separators", plain, "9:9, 1:101;2:2\n3:3\t4:4", 1, 101, norm},
		{"later separator entries", plain, "9:9, 1:5;2:2\n3:3 4:4", 4, 4, norm},
		{"system wildcard", plain, "1:*", 1, 777, norm},
		{"system wildcard other system", plain, "1:*", 2, 777, plain},
		{"exact beats wildcard", plain, "1:*, 1:101:loud", 1, 101, loud},
		{"exact beats wildcard (other order)", plain, "1:101:loud, 1:*", 1, 101, loud},
		{"wildcard loud", plain, "1:*:loud", 1, 5, loud},
		{"global norm overridden by loud entry", norm, "1:101:loud", 1, 101, loud},
		{"global loud overridden by plain entry", loud, "1:101", 1, 101, norm},
		{"garbage ignored", plain, "abc, 1:, :2, 1:x, 1:2:3:4, 1", 1, 2, plain},
		{"huge number ignored", plain, "99999999999:1", 1, 1, plain},
	}
	for _, c := range cases {
		if got := effectiveAudioMode(c.global, c.list, c.sys, c.tg); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}
