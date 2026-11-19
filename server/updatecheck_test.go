package main

import (
	"encoding/json"
	"testing"
)

// Release data shaped like GitHub's API (digests shortened).
const testReleases = `[
 {"tag_name":"6.6.3-v1.3","name":"Rdio-Scanner Lel Edition v1.3","published_at":"2026-11-17T10:47:50Z","html_url":"https://x/v1.3","assets":[{"name":"rdio-scanner_lel_v1.3.exe","size":21673472,"digest":"sha256:AAA3","browser_download_url":"https://x/v1.3.exe"}]},
 {"tag_name":"6.6.3-v1.2","name":"Rdio-Scanner Lel Edition v1.2","published_at":"2026-11-12T05:40:52Z","html_url":"https://x/v1.2","assets":[{"name":"rdio-scanner_lel_v1.2.exe","size":21667840,"digest":"sha256:aaa2","browser_download_url":"https://x/v1.2.exe"}]},
 {"tag_name":"v6.6.3","name":"","published_at":"2026-11-11T08:57:36Z","html_url":"https://x/v1","assets":[{"name":"notes.txt","digest":"sha256:txt"},{"name":"rdio-scanner_lel_v1.exe","size":1,"digest":"sha256:aaa0","browser_download_url":"https://x/v1.exe"}]}
]`

func TestEvaluateUpdate(t *testing.T) {
	var rel []ghRelease
	if err := json.Unmarshal([]byte(testReleases), &rel); err != nil {
		t.Fatal(err)
	}

	got := evaluateUpdate(rel, "aaa3") // newest release (digest compared case-insensitively)
	if got.Status != "current" || got.Current.Tag != "6.6.3-v1.3" || got.Latest != nil {
		t.Fatalf("current: %+v", got)
	}

	got = evaluateUpdate(rel, "AAA2") // older release
	if got.Status != "available" || got.Current.Tag != "6.6.3-v1.2" || got.Latest.Tag != "6.6.3-v1.3" ||
		got.Latest.AssetName != "rdio-scanner_lel_v1.3.exe" || got.Latest.AssetURL != "https://x/v1.3.exe" || got.Latest.AssetSize != 21673472 {
		t.Fatalf("available: %+v", got)
	}

	got = evaluateUpdate(rel, "aaa0") // oldest, whose release has a non-exe asset listed first
	if got.Status != "available" || got.Current.Name != "v6.6.3" {
		t.Fatalf("oldest: %+v", got)
	}

	if got = evaluateUpdate(rel, "ffff"); got.Status != "unknown" { // local build, not uploaded
		t.Fatalf("unknown: %+v", got)
	}
	if got = evaluateUpdate(rel, ""); got.Status != "unknown" {
		t.Fatalf("empty hash: %+v", got)
	}
	if got = evaluateUpdate(nil, "aaa3"); got.Status != "unknown" {
		t.Fatalf("no releases: %+v", got)
	}

	// Drafts and prereleases are ignored, whatever their date.
	rel = append([]ghRelease{{Tag: "9.9", Published: "2030-01-01T00:00:00Z", Prerelease: true, Assets: []ghAsset{{Name: "a.exe", Digest: "sha256:pre"}}}}, rel...)
	if got = evaluateUpdate(rel, "aaa3"); got.Status != "current" {
		t.Fatalf("prerelease must be ignored: %+v", got)
	}

	// Order of the input does not matter: newest is chosen by published date.
	rel[1], rel[3] = rel[3], rel[1]
	if got = evaluateUpdate(rel, "aaa2"); got.Status != "available" || got.Latest.Tag != "6.6.3-v1.3" {
		t.Fatalf("unsorted input: %+v", got)
	}
}
