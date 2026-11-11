package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

// v4: read-only statistics. Results are cached in memory for 5 minutes per
// (range, system) so repeated views never hit the database.

type statsCacheEntry struct {
	at   time.Time
	data []byte
}

var (
	statsCache      = map[string]statsCacheEntry{}
	statsCacheMutex sync.Mutex
)

const statsCacheTTL = 5 * time.Minute

func statsRange(name string, now time.Time) (time.Time, time.Time, time.Time, time.Time) {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var from, to time.Time
	switch name {
	case "today":
		from, to = day, now
	case "yesterday":
		from, to = day.AddDate(0, 0, -1), day
	case "7d":
		from, to = now.AddDate(0, 0, -7), now
	case "30d":
		from, to = now.AddDate(0, 0, -30), now
	default:
		from, to = now.Add(-24*time.Hour), now
	}
	d := to.Sub(from)
	return from, to, from.Add(-d), from
}

func (controller *Controller) countCalls(from, to time.Time, system uint) uint {
	var n uint
	q := "select count(*) from `rdioScannerCalls` where `dateTime` between ? and ?"
	args := []any{from.UTC().Format(controller.Database.DateTimeFormat), to.UTC().Format(controller.Database.DateTimeFormat)}
	if system > 0 {
		q += " and `system` = ?"
		args = append(args, system)
	}
	controller.Database.Sql.QueryRow(q, args...).Scan(&n)
	return n
}

func (controller *Controller) BuildStatistics(rangeName string, system uint) ([]byte, error) {
	key := fmt.Sprintf("%s|%d", rangeName, system)
	statsCacheMutex.Lock()
	if e, ok := statsCache[key]; ok && time.Since(e.at) < statsCacheTTL {
		statsCacheMutex.Unlock()
		return e.data, nil
	}
	statsCacheMutex.Unlock()

	db := controller.Database
	now := time.Now()
	from, to, pfrom, pto := statsRange(rangeName, now)

	q := "select `dateTime`, `system`, `talkgroup` from `rdioScannerCalls` where `dateTime` between ? and ?"
	args := []any{from.UTC().Format(db.DateTimeFormat), to.UTC().Format(db.DateTimeFormat)}
	if system > 0 {
		q += " and `system` = ?"
		args = append(args, system)
	}
	rows, err := db.Sql.Query(q, args...)
	if err != nil {
		return nil, err
	}

	hourly := make([]uint, 24)
	tgCount := map[[2]uint]uint{}
	var total uint
	for rows.Next() {
		var (
			dt      any
			sys, tg uint
		)
		if rows.Scan(&dt, &sys, &tg) != nil {
			continue
		}
		t, err := db.ParseDateTime(dt)
		if err != nil {
			continue
		}
		total++
		hourly[t.Local().Hour()]++
		tgCount[[2]uint{sys, tg}]++
	}
	rows.Close()

	// heatmap: always the last 7 days, [weekday][hour]
	heat := make([][]uint, 7)
	for i := range heat {
		heat[i] = make([]uint, 24)
	}
	heatCounts := map[[2]int]map[[2]uint]uint{}
	heatDates := make([]string, 7)
	startDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -6)
	for i := 0; i < 7; i++ {
		d := startDay.AddDate(0, 0, i)
		heatDates[int(d.Weekday())] = d.Format("2006-01-02")
	}
	hq := "select `dateTime`, `system`, `talkgroup` from `rdioScannerCalls` where `dateTime` >= ? and `dateTime` <= ?"
	hargs := []any{startDay.UTC().Format(db.DateTimeFormat), now.UTC().Format(db.DateTimeFormat)}
	if system > 0 {
		hq += " and `system` = ?"
		hargs = append(hargs, system)
	}
	if hrows, err := db.Sql.Query(hq, hargs...); err == nil {
		for hrows.Next() {
			var dt any
			var sys, tg uint
			if hrows.Scan(&dt, &sys, &tg) == nil {
				if t, err := db.ParseDateTime(dt); err == nil {
					lt := t.Local()
					heat[int(lt.Weekday())][lt.Hour()]++
					cell := [2]int{int(lt.Weekday()), lt.Hour()}
					if heatCounts[cell] == nil {
						heatCounts[cell] = map[[2]uint]uint{}
					}
					heatCounts[cell][[2]uint{sys, tg}]++
				}
			}
		}
		hrows.Close()
	}

	type tgStat struct {
		System      uint   `json:"system"`
		SystemLabel string `json:"systemLabel"`
		Talkgroup   uint   `json:"talkgroup"`
		Label       string `json:"label"`
		Name        string `json:"name"`
		Count       uint   `json:"count"`
	}
	makeTop := func(counts map[[2]uint]uint) []tgStat {
		top := []tgStat{}
		for k, c := range counts {
			s := tgStat{System: k[0], Talkgroup: k[1], Count: c, Label: strconv.Itoa(int(k[1]))}
			if sys, ok := controller.Systems.GetSystem(k[0]); ok {
				s.SystemLabel = sys.Label
				if tg, ok := sys.Talkgroups.GetTalkgroup(k[1]); ok {
					s.Label, s.Name = tg.Label, tg.Name
				}
			}
			top = append(top, s)
		}
		sort.Slice(top, func(i, j int) bool {
			if top[i].Count != top[j].Count {
				return top[i].Count > top[j].Count
			}
			if top[i].System != top[j].System {
				return top[i].System < top[j].System
			}
			return top[i].Talkgroup < top[j].Talkgroup
		})
		if len(top) > 10 {
			top = top[:10]
		}

		return top
	}
	top := makeTop(tgCount)
	heatTop := make([][][]tgStat, 7)
	for day := range heatTop {
		heatTop[day] = make([][]tgStat, 24)
		for hour := range heatTop[day] {
			heatTop[day][hour] = makeTop(heatCounts[[2]int{day, hour}])
		}
	}
	peak := 0
	for h, c := range hourly {
		if c > hourly[peak] {
			peak = h
		}
	}

	systems := []map[string]any{}
	for _, s := range controller.Systems.List {
		systems = append(systems, map[string]any{"id": s.Id, "label": s.Label})
	}

	b, err := json.Marshal(map[string]any{
		"range":     rangeName,
		"total":     total,
		"previous":  controller.countCalls(pfrom, pto, system),
		"hourly":    hourly,
		"peakHour":  peak,
		"top":       top,
		"heatmap":   heat,
		"heatTop":   heatTop,
		"heatDates": heatDates,
		"systems":   systems,
		"cachedAt":  now,
	})
	if err != nil {
		return nil, err
	}
	statsCacheMutex.Lock()
	statsCache[key] = statsCacheEntry{at: now, data: b}
	statsCacheMutex.Unlock()
	return b, nil
}

// StatisticsHandler serves /api/statistics. Allowed for admins, or for anyone
// when Options.PublicStatistics is enabled.
func (controller *Controller) StatisticsHandler(w http.ResponseWriter, r *http.Request) {
	if !controller.Options.PublicStatistics && !controller.Admin.ValidateToken(controller.Admin.GetAuthorization(r)) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	rangeName := r.URL.Query().Get("range")
	switch rangeName {
	case "today", "yesterday", "24h", "7d", "30d":
	default:
		rangeName = "24h"
	}
	var system uint
	if v, err := strconv.Atoi(r.URL.Query().Get("system")); err == nil && v > 0 {
		system = uint(v)
	}
	b, err := controller.BuildStatistics(rangeName, system)
	if err != nil {
		controller.Logs.LogEvent(LogLevelError, fmt.Sprintf("statistics: %v", err))
		w.WriteHeader(http.StatusExpectationFailed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}
