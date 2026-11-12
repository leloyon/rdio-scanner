# Rdio Scanner custom v5.1 — official 6.6.3 base

v5.1 corrects the Audio Conversion caption and matches the theme selector and
its menu to the grey scanner. Audio processing and database schema are unchanged.
Measured quiet 71-second MP3 test: input -56.05 LUFS; no normalization -56.45;
standard normalization -24.45; loud normalization -16.45 (FFmpeg 8.1.2).
The user's installed FFmpeg and recordings have not been tested.

## Install safely
Stop the existing scanner and back up rdio-scanner.db and rdio-scanner.ini first.
Keep the backup outside the active scanner folder. Place rdio-scanner_v5.1.exe in
the existing scanner folder and launch it using the same options as before.
Do not run old and new programs against the same database simultaneously.
The displayed upstream version stays 6.6.3; v5 names this custom release.
If the browser shows old controls, hard-refresh or clear this site's cache.

## Included
- Existing v4 duplicate/file-stability fixes, defaults 3584 bytes and 180 seconds.
- Existing admin statistics, public-statistics toggle (off by default), maintenance tools and mobile-prompt removal.
- Seventeen browser display themes, saved locally; playback-volume slider below buttons.
- Live browser tab call title, restored on pause/silence; STATS uses the stock keypad beep setting.
- Click a heatmap square for its date/hour top 10 talkgroups; Reset restores the range ranking.
  Heatmap covers today and the preceding six calendar days in the server's timezone.
  Drilldown data shares the five-minute statistics cache, with no extra request on each click.
- Existing global Audio Conversion choices now encode mono MP3 at 32 kbps,
  22050 Hz, retaining the stock standard/loud normalization filters.
  Disabled leaves input unchanged. FFmpeg with libmp3lame remains required;
  normalization needs FFmpeg 4.3 or newer, as in upstream.
  Failures keep original audio and report the conversion error.
- Existing stored recordings are not re-encoded or repaired retroactively.
- No per-talkgroup normalization or squelch sound changes.

## Build the complete source ZIP
The ZIP contains the complete already-modified official release source, not
only patches. No downloaded dependencies, generated webapp, or executables
are included. Build the client before building the server (Go embeds webapp).

Linux/WSL: use Go 1.22.5, Node compatible with Angular 13 (Node 16 recommended),
npm, Python 3, and FFmpeg/ffprobe for tests.

```sh
cd client
npm ci
NODE_OPTIONS=--openssl-legacy-provider npm run build
cd ../server
go test ./...
GOOS=windows GOARCH=amd64 go build -o rdio-scanner_v5.1.exe .
```

Windows PowerShell, with Go, Node/npm and FFmpeg on PATH:
```powershell
cd client
npm ci
$env:NODE_OPTIONS='--openssl-legacy-provider'
npm run build
cd ../server
go test ./...
$env:GOOS='windows'
$env:GOARCH='amd64'
go build -o rdio-scanner_v5.1.exe .
```

## Verification and limits
Passed Go tests for inherited duplicate/prune rules, four conversion modes,
71-second synthetic audio, mono/32kbps MP3, decoding, error preservation,
defaults, heatmap ranking/system/hour filters, and cache reuse.
Browser checks passed all seventeen theme choices, saved theme/volume, heatmap
selection/reset, stored-call playback, live title/pause restoration and 390px layout.
An unmodified official 6.6.3 program created a test database; v5 started on a
copy without changing its schema, talkgroup rows, or audio hashes.
Windows binary was cross-compiled but not executed on Windows; the user's
actual database, original failing recording and native apps were not available.
No claim is made that every source of playback failure is eliminated.