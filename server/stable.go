package main

import (
	"os"
	"time"
)

// v4: readStableFile waits until a file stops growing (two equal size checks
// 500ms apart) and retries locked reads, up to Options.DirwatchMaxWait seconds.
// A max wait of 0 disables the wait and reads immediately.
func readStableFile(controller *Controller, p string) ([]byte, error) {
	maxWait := time.Duration(controller.Options.DirwatchMaxWait) * time.Second
	if maxWait == 0 {
		return os.ReadFile(p)
	}
	deadline := time.Now().Add(maxWait)
	var last int64 = -1
	stable := 0
	for time.Now().Before(deadline) {
		fi, err := os.Stat(p)
		if err == nil {
			if fi.Size() == last && fi.Size() > 0 {
				stable++
				if stable >= 2 {
					if b, err := os.ReadFile(p); err == nil {
						return b, nil
					}
					stable = 0
				}
			} else {
				stable = 0
			}
			last = fi.Size()
		}
		time.Sleep(500 * time.Millisecond)
	}
	return os.ReadFile(p)
}
