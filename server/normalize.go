// Per-talkgroup audio normalization (custom v5.1).
//
// The "Normalize Talkgroups" option lists the talkgroups whose new calls are normalized,
// whatever the global Audio Conversion choice is (unless conversion is Disabled).
//
// Entries are separated by commas, semicolons, spaces or new lines:
//
//	1:101         talkgroup 101 of system 1, standard normalization
//	1:101:loud    same, loud normalization
//	1:*           every talkgroup of system 1 (an exact talkgroup entry wins over this)
//
// Invalid entries are ignored. Only new calls are affected.

package main

import (
	"strconv"
	"strings"
	"unicode"
)

// effectiveAudioMode returns the audio conversion mode to use for a call: the global mode,
// unless the call's system/talkgroup is listed in the normalize list.
func effectiveAudioMode(global uint, list string, system, talkgroup uint) uint {
	if global == AUDIO_CONVERSION_DISABLED || strings.TrimSpace(list) == "" {
		return global
	}

	var (
		systemMode uint
		haveSystem bool
	)

	entries := strings.FieldsFunc(list, func(r rune) bool {
		return r == ',' || r == ';' || unicode.IsSpace(r)
	})

	for _, entry := range entries {
		parts := strings.Split(entry, ":")
		if len(parts) < 2 || len(parts) > 3 {
			continue
		}

		sys, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil || uint(sys) != system {
			continue
		}

		mode := uint(AUDIO_CONVERSION_ENABLED_NORM)
		if len(parts) == 3 {
			if !strings.EqualFold(parts[2], "loud") {
				continue
			}
			mode = AUDIO_CONVERSION_ENABLED_LOUD_NORM
		}

		if parts[1] == "*" {
			systemMode, haveSystem = mode, true
			continue
		}

		tg, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil || uint(tg) != talkgroup {
			continue
		}

		return mode
	}

	if haveSystem {
		return systemMode
	}

	return global
}
