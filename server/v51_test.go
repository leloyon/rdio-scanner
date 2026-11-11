package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"strconv"
	"testing"
)

func measuredLoudness(t *testing.T, audio []byte) float64 {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-hide_banner", "-i", "-", "-af", "loudnorm=print_format=json", "-f", "null", "-")
	cmd.Stdin = bytes.NewReader(audio)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("loudness measurement failed: %v", err)
	}
	match := regexp.MustCompile(`"input_i"\s*:\s*"(-?[0-9.]+)"`).FindSubmatch(out)
	if len(match) != 2 {
		t.Fatal("no finite loudness measurement")
	}
	value, err := strconv.ParseFloat(string(match[1]), 64)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestV51MeasuredNormalization(t *testing.T) {
	cmd := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=700:duration=71", "-af", "volume=0.02", "-ac", "1", "-b:a", "32k", "-f", "mp3", "-")
	original, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	input := measuredLoudness(t, original)
	levels := map[uint]float64{}
	for _, mode := range []uint{1, 2, 3} {
		call := &Call{Audio: append([]byte(nil), original...), AudioType: "audio/mpeg", AudioName: "quiet.mp3"}
		if err := NewFFMpeg().Convert(call, NewSystems(), NewTags(), mode); err != nil {
			t.Fatal(err)
		}
		levels[mode] = measuredLoudness(t, call.Audio)
		t.Logf("mode %d: input %.2f LUFS, output %.2f LUFS", mode, input, levels[mode])
	}
	if levels[1] < input-1 || levels[1] > input+1 {
		t.Fatal("conversion without normalization changed loudness")
	}
	if levels[2] < -25.5 || levels[2] > -22.5 {
		t.Fatalf("standard normalization did not reach -24 LUFS: %.2f", levels[2])
	}
	if levels[3] < -17.5 || levels[3] > -14.5 {
		t.Fatalf("loud normalization did not reach -16 LUFS: %.2f", levels[3])
	}
	if levels[3]-levels[2] < 6 {
		t.Fatal("loud mode is not measurably louder than standard mode")
	}
}
