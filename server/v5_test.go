package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestV5LongMP3Normalization(t *testing.T) {
	cmd := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=700:duration=71", "-af", "volume=0.1", "-ac", "1", "-b:a", "32k", "-f", "mp3", "-")
	original, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []uint{0, 1, 2, 3} {
		call := &Call{Audio: append([]byte(nil), original...), AudioType: "audio/mpeg", AudioName: "long.mp3"}
		if err := NewFFMpeg().Convert(call, NewSystems(), NewTags(), mode); err != nil {
			t.Fatal(err)
		}
		if mode == 0 && !bytes.Equal(call.Audio, original) {
			t.Fatal("disabled changed audio")
		}
		if call.AudioType != "audio/mpeg" || call.AudioName != "long.mp3" {
			t.Fatal("wrong output type")
		}
		probe := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_name,channels,bit_rate", "-of", "json", "-i", "-")
		probe.Stdin = bytes.NewReader(call.Audio)
		b, err := probe.Output()
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			Streams []struct {
				Codec    string `json:"codec_name"`
				Channels int    `json:"channels"`
				Bitrate  string `json:"bit_rate"`
			}
		}
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Streams) != 1 || got.Streams[0].Codec != "mp3" || got.Streams[0].Channels != 1 || got.Streams[0].Bitrate != "32000" {
			t.Fatal(string(b))
		}
		decode := exec.Command("ffmpeg", "-v", "error", "-i", "-", "-f", "null", "-")
		decode.Stdin = bytes.NewReader(call.Audio)
		if b, err := decode.CombinedOutput(); err != nil {
			t.Fatal(err, string(b))
		}
		if mode == 2 {
			if p := os.Getenv("RDIO_TEST_AUDIO_OUT"); p != "" {
				if err := os.WriteFile(p, call.Audio, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestV5ConversionFailureKeepsAudio(t *testing.T) {
	c := &Call{Audio: []byte("invalid recording"), AudioName: "original.mp3", AudioType: "audio/mpeg"}
	b := append([]byte(nil), c.Audio...)
	if err := NewFFMpeg().Convert(c, NewSystems(), NewTags(), 2); err == nil {
		t.Fatal("expected conversion error")
	}
	if !bytes.Equal(b, c.Audio) || c.AudioName != "original.mp3" {
		t.Fatal("lost original")
	}
}

func TestV5OptionDefaults(t *testing.T) {
	o := NewOptions().FromMap(map[string]any{})
	if o.DuplicateBlankAudioSize != 3584 {
		t.Fatal(o.DuplicateBlankAudioSize)
	}
	if o.DirwatchMaxWait != 180 {
		t.Fatal(o.DirwatchMaxWait)
	}
	if o.PublicStatistics {
		t.Fatal("public stats must default off")
	}
}

func TestV5HeatmapRanking(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("create table rdioScannerCalls(dateTime TEXT, system INTEGER, talkgroup INTEGER)"); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	d := time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 0, 0, now.Location()).AddDate(0, 0, -1)
	format := "2006-01-02 15:04:05.000 -07:00"
	for _, r := range []struct {
		date    time.Time
		sys, tg int
	}{{d, 1, 101}, {d, 1, 101}, {d, 1, 202}, {d.Add(time.Hour), 1, 303}, {d, 2, 404}} {
		if _, err := db.Exec("insert into rdioScannerCalls values(?,?,?)", r.date.UTC().Format(format), r.sys, r.tg); err != nil {
			t.Fatal(err)
		}
	}
	c := &Controller{Database: &Database{Sql: db, DateTimeFormat: format}, Systems: NewSystems()}
	statsCacheMutex.Lock()
	statsCache = map[string]statsCacheEntry{}
	statsCacheMutex.Unlock()
	b, err := c.BuildStatistics("7d", 1)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Total     uint
		HeatDates []string
		Heatmap   [][]uint
		HeatTop   [][][]struct {
			Talkgroup uint
			Count     uint
		}
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	day := int(d.Weekday())
	cell := got.HeatTop[day][15]
	if got.Total != 4 || got.Heatmap[day][15] != 3 || got.HeatDates[day] != d.Format("2006-01-02") || len(cell) != 2 || cell[0].Talkgroup != 101 || cell[0].Count != 2 || cell[1].Talkgroup != 202 {
		t.Fatal(string(b))
	}
	if len(got.HeatTop[day][16]) != 1 || got.HeatTop[day][16][0].Talkgroup != 303 {
		t.Fatal("hours leaked")
	}
	cached, err := c.BuildStatistics("7d", 1)
	if err != nil || !bytes.Equal(cached, b) {
		t.Fatal("cache failed")
	}
}
