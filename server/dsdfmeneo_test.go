package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestParseDSDFMENeoMeta(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no tz database")
	}
	saved := time.Local
	time.Local = loc
	defer func() { time.Local = saved }()

	cases := []struct {
		name string
		path string
		tg   uint
		src  uint
		when time.Time
	}{
		{"dsd-fme p25", filepath.Join("rec", "dsd", "20261116_220202_62120_P25_BEE0034834D_3_78_GROUP_TGT_26035_SRC_4090280.wav"), 26035, 4090280, time.Date(2026, 11, 17, 3, 2, 2, 0, time.UTC)},
		{"dsd-neo dmr", filepath.Join("rec", "20261124_230025_30560_DMR_CC_10_GROUP_TGT_400201_SRC_4009996.wav"), 400201, 4009996, time.Date(2026, 11, 25, 4, 0, 25, 0, time.UTC)},
		{"no extension", "20261124_230025_30560_DMR_CC_10_GROUP_TGT_400201_SRC_4009996", 400201, 4009996, time.Date(2026, 11, 25, 4, 0, 25, 0, time.UTC)},
		{"summer (daylight time)", "20260704_120000_1_P25_X_GROUP_TGT_7_SRC_8.wav", 7, 8, time.Date(2026, 7, 4, 16, 0, 0, 0, time.UTC)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			call := NewCall()
			if err := ParseDSDFMENeoMeta(call, c.path); err != nil {
				t.Fatal(err)
			}
			if call.Talkgroup != c.tg {
				t.Errorf("talkgroup = %d, want %d", call.Talkgroup, c.tg)
			}
			if call.Source != c.src {
				t.Errorf("source = %v, want %d", call.Source, c.src)
			}
			if !call.DateTime.Equal(c.when) {
				t.Errorf("time = %v, want %v", call.DateTime, c.when)
			}
		})
	}

	for _, bad := range []string{"TEMP_20261116_220202_1234", "notes.wav", "20261116_220202_1_M17_GROUP_TGT_ABC_SRC_DEF.wav"} {
		if err := ParseDSDFMENeoMeta(NewCall(), bad); err == nil {
			t.Errorf("%q should not parse", bad)
		}
	}
}

func TestParseDSDFMENeoCallsigns(t *testing.T) {
	// numeric target kept, callsign source gets a stable id and a unit label
	call := NewCall()
	call.Talkgroup = 99
	if err := ParseDSDFMENeoMeta(call, "20261124_230025_1_M17_X_TGT_ALL_SRC_W1ABC.wav"); err != nil {
		t.Fatal(err)
	}
	if call.Talkgroup != 99 {
		t.Errorf("talkgroup = %d, want default 99", call.Talkgroup)
	}
	id, ok := call.Source.(uint)
	if !ok || id < 16777216 {
		t.Fatalf("source = %v, want a generated id", call.Source)
	}
	call2 := NewCall()
	call2.Talkgroup = 99
	if err := ParseDSDFMENeoMeta(call2, "20261124_230026_1_M17_X_TGT_ALL_SRC_w1abc.wav"); err != nil {
		t.Fatal(err)
	}
	if call2.Source != call.Source {
		t.Errorf("callsign id not stable: %v vs %v", call.Source, call2.Source)
	}
	if call.units == nil {
		t.Errorf("unit label missing")
	}

	// callsign target with no default talkgroup must fail
	if err := ParseDSDFMENeoMeta(NewCall(), "20261124_230025_1_YSF_X_TGT_ALL_SRC_W1ABC.wav"); err == nil {
		t.Errorf("expected error without default talkgroup")
	}
}
