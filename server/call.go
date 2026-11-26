// Copyright (C) 2019-2022 Chrystian Huot <chrystian.huot@saubeo.solutions>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>

package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

type Call struct {
	Id             any       `json:"id"`
	Audio          []byte    `json:"audio"`
	AudioName      any       `json:"audioName"`
	AudioType      any       `json:"audioType"`
	DateTime       time.Time `json:"dateTime"`
	Frequencies    any       `json:"frequencies"`
	Frequency      any       `json:"frequency"`
	Patches        any       `json:"patches"`
	Source         any       `json:"source"`
	Sources        any       `json:"sources"`
	System         uint      `json:"system"`
	Talkgroup      uint      `json:"talkgroup"`
	systemLabel    any
	talkgroupGroup any
	talkgroupLabel any
	talkgroupName  any
	talkgroupTag   any
	units          any
	rawSize        int // audio size before conversion (conversion pads short calls to 3 s)
}

func NewCall() *Call {
	return &Call{
		Frequencies: []map[string]any{},
		Patches:     []uint{},
		Sources:     []map[string]any{},
	}
}

func (call *Call) IsValid() (ok bool, err error) {
	ok = true

	if len(call.Audio) <= 44 {
		ok = false
		err = errors.New("no audio")
	}

	if call.DateTime.Unix() == 0 {
		ok = false
		err = errors.New("no datetime")
	}

	if call.System < 1 {
		ok = false
		err = errors.New("no system")
	}

	if call.Talkgroup < 1 {
		ok = false
		err = errors.New("no talkgroup")
	}

	return ok, err
}

func (call *Call) MarshalJSON() ([]byte, error) {
	audio := fmt.Sprintf("%v", call.Audio)
	audio = strings.ReplaceAll(audio, " ", ",")

	return json.Marshal(map[string]any{
		"id": call.Id,
		"audio": map[string]any{
			"data": json.RawMessage(audio),
			"type": "Buffer",
		},
		"audioName":   call.AudioName,
		"audioType":   call.AudioType,
		"dateTime":    call.DateTime.Format(time.RFC3339),
		"frequencies": call.Frequencies,
		"frequency":   call.Frequency,
		"patches":     call.Patches,
		"source":      call.Source,
		"sources":     call.Sources,
		"system":      call.System,
		"talkgroup":   call.Talkgroup,
	})
}

func (call *Call) ToJson() (string, error) {
	if b, err := json.Marshal(call); err == nil {
		return string(b), nil
	} else {
		return "", fmt.Errorf("call.tojson: %v", err)
	}
}

type Calls struct {
	mutex   sync.Mutex
	rawSize map[uint]int // recent call id -> audio size before conversion
}

func NewCalls() *Calls {
	return &Calls{
		mutex:   sync.Mutex{},
		rawSize: map[uint]int{},
	}
}

// scaleToStored turns an existing call's real size into the units of the new
// call's stored (converted) size so the two can be compared, or returns the
// stored size when the real sizes are not known.
func scaleToStored(existingRaw, newRaw, newStored, existingStored int) int {
	if existingRaw <= 0 || newRaw <= 0 || newStored <= 0 {
		return existingStored
	}
	return int(float64(existingRaw) * float64(newStored) / float64(newRaw))
}

// NoteRawSize remembers how long a stored call really was. Conversion pads every
// call to at least 3 seconds, so two copies of a short call look the same size
// once stored, and the longer one can't be told from the shorter one.
func (calls *Calls) NoteRawSize(id uint, size int) {
	if size <= 0 {
		return
	}
	calls.mutex.Lock()
	defer calls.mutex.Unlock()
	if calls.rawSize == nil {
		calls.rawSize = map[uint]int{}
	}
	calls.rawSize[id] = size
	if len(calls.rawSize) > 2000 {
		for k := range calls.rawSize {
			if k+1000 < id {
				delete(calls.rawSize, k)
			}
		}
	}
}

// DuplicateAction is the v4 duplicate decision.
type DuplicateAction int

const (
	DuplicateNone    DuplicateAction = iota // not a duplicate, store it
	DuplicateReject                         // a longer copy already exists
	DuplicateReplace                        // new call is longer, store it and delete the old copies
)

// Two calls are copies of one transmission only when their sizes are within this
// ratio, unless they also come from the same radio (see below).
const duplicateSizeRatio = 0.6

// duplicateGate makes "check for a duplicate, then store" one step, so two
// copies finishing at the same moment cannot both pass the check.
var duplicateGate sync.Mutex

// callSourceID returns the radio ID of a call, or 0 when unknown.
func callSourceID(v any) uint {
	switch n := v.(type) {
	case uint:
		return n
	case int:
		if n > 0 {
			return uint(n)
		}
	case uint64:
		return uint(n)
	case int64:
		if n > 0 {
			return uint(n)
		}
	case float64:
		if n > 0 {
			return uint(n)
		}
	}
	return 0
}

// CheckDuplicate keeps the longest copy of a transmission and keeps different
// audio formats separate. A short key-up next to a long conversation on the
// same talkgroup is treated as a different transmission and both are kept.
//
// The third value is a one line explanation of what was compared and why, for
// the log. It is empty when no other call on the same talkgroup is close in time.
func (calls *Calls) CheckDuplicate(call *Call, msTimeFrame uint, db *Database) (DuplicateAction, []uint, string) {
	calls.mutex.Lock()
	defer calls.mutex.Unlock()

	d := time.Duration(msTimeFrame) * time.Millisecond

	// look wider than the window so a copy that just missed it shows up in the log
	wide := 10 * time.Second
	if 3*d > wide {
		wide = 3 * d
	}
	from := call.DateTime.Add(-wide)
	to := call.DateTime.Add(wide)

	query := fmt.Sprintf("select `id`, coalesce(length(`audio`), 0), coalesce(`audioType`, ''), coalesce(`source`, 0), `dateTime` from `rdioScannerCalls` where (`dateTime` between '%v' and '%v') and `system` = %v and `talkgroup` = %v order by `dateTime`", from, to, call.System, call.Talkgroup)
	rows, err := db.Sql.Query(query)
	if err != nil {
		return DuplicateNone, nil, ""
	}
	defer rows.Close()

	newSize := len(call.Audio)
	newType := ""
	if v, ok := call.AudioType.(string); ok {
		newType = v
	}
	newSource := callSourceID(call.Source)

	notes := []string{}

	action, ids, reasons := decideDuplicateExplain(newSize, newType, func(yield func(id uint, size int, atype string, source int) bool) {
		for rows.Next() {
			var (
				id    uint
				size  int
				atype string
				src   float64
				dt    any
			)
			if rows.Scan(&id, &size, &atype, &src, &dt) != nil {
				continue
			}

			gap := time.Duration(0)
			if t, err := db.ParseDateTime(dt); err == nil {
				gap = t.Sub(call.DateTime)
			}
			absGap := gap
			if absGap < 0 {
				absGap = -absGap
			}

			info := fmt.Sprintf("#%d %+dms size=%d type=%s radio=%d", id, gap.Milliseconds(), size, atype, uint(src))

			if absGap > d {
				notes = append(notes, fmt.Sprintf("%s: outside the %dms window, not compared", info, msTimeFrame))
				continue
			}

			// compare the real lengths when both are known, scaled to the stored size
			if raw, ok := calls.rawSize[id]; ok {
				size = scaleToStored(raw, call.rawSize, newSize, size)
			}
			source := 0
			if newSource > 0 && src > 0 {
				if uint(src) == newSource {
					source = 1
				} else {
					source = -1
				}
			}
			notes = append(notes, info) // the reason is added below, in order
			if !yield(id, size, atype, source) {
				return
			}
		}
	})

	if len(notes) == 0 {
		return action, ids, ""
	}

	// attach each decision reason to the compared candidate it belongs to
	r := 0
	for i := range notes {
		if !strings.Contains(notes[i], "window, not compared") && r < len(reasons) {
			notes[i] += ": " + reasons[r]
			r++
		}
	}

	decision := map[DuplicateAction]string{DuplicateNone: "stored", DuplicateReject: "rejected", DuplicateReplace: "replaces older copy"}[action]

	return action, ids, fmt.Sprintf("duplicate check (window %dms): new size=%d (real %d) type=%s radio=%d -> %s; nearby: %s", msTimeFrame, newSize, call.rawSize, newType, newSource, decision, strings.Join(notes, "; "))
}

// decideDuplicate holds the pure decision rules (unit tested).
func decideDuplicate(newSize int, newType string, each func(func(id uint, size int, atype string) bool)) (DuplicateAction, []uint) {
	return decideDuplicateSrc(newSize, newType, func(yield func(uint, int, string, int) bool) {
		each(func(id uint, size int, atype string) bool { return yield(id, size, atype, 0) })
	})
}

// decideDuplicateSrc is decideDuplicate that also knows about the radio:
// source is 1 when an existing call came from the same radio, -1 when it came
// from a different radio, 0 when either is unknown. The same radio on the same
// talkgroup at the same moment is the same transmission even when one site heard
// only part of it, so the size rule is skipped. A different radio is a different
// transmission, so both are kept.
func decideDuplicateSrc(newSize int, newType string, each func(func(id uint, size int, atype string, source int) bool)) (DuplicateAction, []uint) {
	action, ids, _ := decideDuplicateExplain(newSize, newType, each)
	return action, ids
}

// decideDuplicateExplain is decideDuplicateSrc that also returns, for each
// candidate it was given (in order), why it was kept apart or counted as a copy.
func decideDuplicateExplain(newSize int, newType string, each func(func(id uint, size int, atype string, source int) bool)) (DuplicateAction, []uint, []string) {
	replace := []uint{}
	reasons := []string{}
	reject := false
	each(func(id uint, size int, atype string, source int) bool {
		if source < 0 {
			reasons = append(reasons, "different radio, kept both")
			return true
		}
		sameSource := source > 0
		if newType != "" && atype != "" && atype != newType {
			reasons = append(reasons, fmt.Sprintf("different format (%s vs %s), kept both", atype, newType))
			return true
		}
		small, big := size, newSize
		if small > big {
			small, big = big, small
		}
		if !sameSource && big > 0 && float64(small)/float64(big) < duplicateSizeRatio {
			reasons = append(reasons, fmt.Sprintf("sizes too different (%d vs %d, below %.0f%%), kept both", size, newSize, duplicateSizeRatio*100))
			return true
		}
		if newSize > size {
			replace = append(replace, id)
			reasons = append(reasons, fmt.Sprintf("same transmission, new copy is longer (%d > %d), replaces it", newSize, size))
			return true
		}
		reasons = append(reasons, fmt.Sprintf("same transmission, stored copy is as long or longer (%d >= %d), new copy rejected", size, newSize))
		reject = true
		return false
	})
	if reject {
		return DuplicateReject, nil, reasons
	}
	if len(replace) > 0 {
		return DuplicateReplace, replace, reasons
	}
	return DuplicateNone, nil, reasons
}

// duplicateV8Window is the fixed window upstream v8 uses (its time frame option is retired).
const duplicateV8Window = 500 * time.Millisecond

// CheckDuplicateV8 follows upstream v8: any other call on the same system and
// talkgroup within +/-500 ms makes the new call a duplicate, the first copy wins,
// and size, format and radio are not looked at. v8 also requires the same site,
// but this version has no site on a call, so every call counts as the same site.
// The second value lists what was found, for the log.
func (calls *Calls) CheckDuplicateV8(call *Call, db *Database) (bool, string) {
	calls.mutex.Lock()
	defer calls.mutex.Unlock()

	from := call.DateTime.Add(-duplicateV8Window)
	to := call.DateTime.Add(duplicateV8Window)

	query := fmt.Sprintf("select `id`, coalesce(length(`audio`), 0), coalesce(`source`, 0), `dateTime` from `rdioScannerCalls` where (`dateTime` between '%v' and '%v') and `system` = %v and `talkgroup` = %v order by `dateTime`", from, to, call.System, call.Talkgroup)
	rows, err := db.Sql.Query(query)
	if err != nil {
		return false, ""
	}
	defer rows.Close()

	found := []string{}
	for rows.Next() {
		var (
			id   uint
			size int
			src  float64
			dt   any
		)
		if rows.Scan(&id, &size, &src, &dt) != nil {
			continue
		}
		gap := time.Duration(0)
		if t, err := db.ParseDateTime(dt); err == nil {
			gap = t.Sub(call.DateTime)
		}
		found = append(found, fmt.Sprintf("#%d %+dms size=%d radio=%d", id, gap.Milliseconds(), size, uint(src)))
	}

	if len(found) == 0 {
		return false, ""
	}

	return true, fmt.Sprintf("v8 rules (fixed %dms window, first copy wins): new size=%d radio=%d; matched %s", duplicateV8Window.Milliseconds(), len(call.Audio), callSourceID(call.Source), strings.Join(found, "; "))
}

func (calls *Calls) DeleteCalls(ids []uint, db *Database) {
	calls.mutex.Lock()
	defer calls.mutex.Unlock()
	for _, id := range ids {
		db.Sql.Exec("delete from `rdioScannerCalls` where `id` = ?", id)
	}
}

func (calls *Calls) GetCall(id uint, db *Database) (*Call, error) {
	var (
		audioName   sql.NullString
		audioType   sql.NullString
		dateTime    any
		frequency   sql.NullFloat64
		source      sql.NullFloat64
		frequencies string
		patches     string
		sources     string
		t           time.Time
	)

	calls.mutex.Lock()
	defer calls.mutex.Unlock()

	call := Call{Id: id}

	query := fmt.Sprintf("select `audio`, `audioName`, `audioType`, `DateTime`, `frequencies`, `frequency`, `patches`, `source`, `sources`, `system`, `talkgroup` from `rdioScannerCalls` where `id` = %v", id)
	err := db.Sql.QueryRow(query).Scan(&call.Audio, &audioName, &audioType, &dateTime, &frequencies, &frequency, &patches, &source, &sources, &call.System, &call.Talkgroup)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("getcall: %v, %v", err, query)
	}

	if audioName.Valid {
		call.AudioName = audioName.String
	}

	if audioType.Valid {
		call.AudioType = audioType.String
	}

	if frequency.Valid && frequency.Float64 > 0 {
		call.Frequency = uint(frequency.Float64)
	}

	if t, err = db.ParseDateTime(dateTime); err == nil {
		call.DateTime = t
	} else {
		call.DateTime = time.Time{}
	}

	if len(frequencies) > 0 {
		if err = json.Unmarshal([]byte(frequencies), &call.Frequencies); err != nil {
			call.Frequencies = []any{}
		}
	}

	if len(patches) > 0 {
		if err = json.Unmarshal([]byte(patches), &call.Patches); err != nil {
			call.Patches = []any{}
		}
	}

	if source.Valid && source.Float64 > 0 {
		call.Source = uint(source.Float64)
	}

	if len(sources) > 0 {
		if err = json.Unmarshal([]byte(sources), &call.Sources); err != nil {
			call.Sources = []any{}
		}
	}

	return &call, nil
}

func (calls *Calls) Prune(db *Database, pruneDays uint) error {
	calls.mutex.Lock()
	defer calls.mutex.Unlock()

	date := time.Now().Add(-24 * time.Hour * time.Duration(pruneDays)).Format(db.DateTimeFormat)
	_, err := db.Sql.Exec("delete from `rdioScannerCalls` where `dateTime` < ?", date)

	return err
}

func (calls *Calls) Search(searchOptions *CallsSearchOptions, client *Client) (*CallsSearchResults, error) {
	const (
		ascOrder  = "asc"
		descOrder = "desc"
	)

	var (
		dateTime any
		err      error
		id       sql.NullFloat64
		limit    uint
		offset   uint
		order    string
		query    string
		rows     *sql.Rows
		t        time.Time
		where    string = "true"
	)

	calls.mutex.Lock()
	defer calls.mutex.Unlock()

	db := client.Controller.Database

	formatError := func(err error) error {
		return fmt.Errorf("calls.search: %v", err)
	}

	searchResults := &CallsSearchResults{
		Options: searchOptions,
		Results: []CallsSearchResult{},
	}

	if client.Access != nil {
		switch v := client.Access.Systems.(type) {
		case []any:
			a := []string{}
			for _, scope := range v {
				var c string
				switch v := scope.(type) {
				case map[string]any:
					switch v["talkgroups"].(type) {
					case []any:
						b := strings.ReplaceAll(fmt.Sprintf("%v", v["talkgroups"]), " ", ", ")
						b = strings.ReplaceAll(b, "[", "(")
						b = strings.ReplaceAll(b, "]", ")")
						c = fmt.Sprintf("(`system` = %v and `talkgroup` in %v)", v["id"], b)
					case string:
						if v["talkgroups"] == "*" {
							c = fmt.Sprintf("`system` = %v", v["id"])
						}
					}
				}
				if len(c) > 0 {
					a = append(a, c)
				}
			}
			where = fmt.Sprintf("(%s)", strings.Join(a, " or "))
		}
	}

	switch v := searchOptions.System.(type) {
	case uint:
		a := []string{
			fmt.Sprintf("`system` = %v", v),
		}
		switch v := searchOptions.Talkgroup.(type) {
		case uint:
			if searchOptions.searchPatchedTalkgroups {
				a = append(a, fmt.Sprintf("`talkgroup` = %v or patches = '%v' or patches like '[%v,%%' or patches like '%%,%v,%%' or patches like '%%,%v]'", v, v, v, v, v))
			} else {
				a = append(a, fmt.Sprintf("`talkgroup` = %v", v))
			}
		}
		where += fmt.Sprintf(" and (%s)", strings.Join(a, " and "))
	}

	switch v := searchOptions.Group.(type) {
	case string:
		a := []string{}
		for id, m := range client.GroupsMap[v] {
			b := strings.ReplaceAll(fmt.Sprintf("%v", m), " ", ", ")
			b = strings.ReplaceAll(b, "[", "(")
			b = strings.ReplaceAll(b, "]", ")")
			a = append(a, fmt.Sprintf("(`system` = %v and `talkgroup` in %v)", id, b))
		}
		if len(a) > 0 {
			where += fmt.Sprintf(" and (%s)", strings.Join(a, " or "))
		}
	}

	switch v := searchOptions.Tag.(type) {
	case string:
		a := []string{}
		for id, m := range client.TagsMap[v] {
			b := strings.ReplaceAll(fmt.Sprintf("%v", m), " ", ", ")
			b = strings.ReplaceAll(b, "[", "(")
			b = strings.ReplaceAll(b, "]", ")")
			a = append(a, fmt.Sprintf("(`system` = %v and `talkgroup` in %v)", id, b))
		}
		if len(a) > 0 {
			where += fmt.Sprintf(" and (%s)", strings.Join(a, " or "))
		}
	}

	query = fmt.Sprintf("select `dateTime` from `rdioScannerCalls` where %v order by `dateTime` asc", where)
	if err = db.Sql.QueryRow(query).Scan(&dateTime); err != nil && err != sql.ErrNoRows {
		return nil, formatError(fmt.Errorf("%v, %v", err, query))
	}

	if t, err = db.ParseDateTime(dateTime); err == nil {
		searchResults.DateStart = t
	}

	query = fmt.Sprintf("select `dateTime` from `rdioScannerCalls` where %v order by `dateTime` desc", where)
	if err = db.Sql.QueryRow(query).Scan(&dateTime); err != nil && err != sql.ErrNoRows {
		return nil, formatError(fmt.Errorf("%v, %v", err, query))
	}

	if t, err = db.ParseDateTime(dateTime); err == nil {
		searchResults.DateStop = t
	} else {
		searchResults.DateStop = time.Now()
	}

	switch v := searchOptions.Sort.(type) {
	case int:
		if v < 0 {
			order = descOrder
		} else {
			order = ascOrder
		}
	default:
		order = ascOrder
	}

	switch v := searchOptions.Date.(type) {
	case time.Time:
		var (
			df    string = client.Controller.Database.DateTimeFormat
			start time.Time
			stop  time.Time
		)

		if order == ascOrder {
			start = time.Date(v.Year(), v.Month(), v.Day(), v.Hour(), v.Minute(), 0, 0, time.UTC)
			stop = start.Add(time.Hour*24 - time.Millisecond)

		} else {
			start = time.Date(v.Year(), v.Month(), v.Day(), v.Hour(), v.Minute(), 0, 0, time.UTC).Add(time.Hour*-24 + time.Millisecond)
			stop = time.Date(v.Year(), v.Month(), v.Day(), v.Hour(), v.Minute(), 0, 0, time.UTC)
		}

		where += fmt.Sprintf(" and (`dateTime` between '%v' and '%v')", start.Format(df), stop.Format(df))
	}

	switch v := searchOptions.Limit.(type) {
	case uint:
		limit = uint(math.Min(float64(500), float64(v)))
	default:
		limit = 200
	}

	switch v := searchOptions.Offset.(type) {
	case uint:
		offset = v
	}

	query = fmt.Sprintf("select count(*) from `rdioScannerCalls` where %v", where)
	if err = db.Sql.QueryRow(query).Scan(&searchResults.Count); err != nil && err != sql.ErrNoRows {
		return nil, formatError(fmt.Errorf("%v, %v", err, query))
	}

	query = fmt.Sprintf("select `id`, `DateTime`, `system`, `talkgroup` from `rdioScannerCalls` where %v order by `dateTime` %v limit %v offset %v", where, order, limit, offset)
	if rows, err = db.Sql.Query(query); err != nil && err != sql.ErrNoRows {
		return nil, formatError(fmt.Errorf("%v, %v", err, query))
	}

	for rows.Next() {
		searchResult := CallsSearchResult{}
		if err = rows.Scan(&id, &dateTime, &searchResult.System, &searchResult.Talkgroup); err != nil {
			break
		}

		if id.Valid && id.Float64 > 0 {
			searchResult.Id = uint(id.Float64)
		}

		if t, err = db.ParseDateTime(dateTime); err == nil {
			searchResult.DateTime = t

		} else {
			continue
		}

		searchResults.Results = append(searchResults.Results, searchResult)
	}

	rows.Close()

	if err != nil {
		return nil, formatError(err)
	}

	return searchResults, err
}

func (calls *Calls) WriteCall(call *Call, db *Database) (uint, error) {
	var (
		b           []byte
		err         error
		frequencies string
		id          int64
		patches     string
		res         sql.Result
		sources     string
	)

	calls.mutex.Lock()
	defer calls.mutex.Unlock()

	formatError := func(err error) error {
		return fmt.Errorf("call.write: %s", err.Error())
	}

	switch v := call.Frequencies.(type) {
	case []map[string]any:
		if b, err = json.Marshal(v); err == nil {
			frequencies = string(b)
		} else {
			return 0, formatError(err)
		}
	}

	switch v := call.Patches.(type) {
	case []uint:
		if b, err = json.Marshal(v); err == nil {
			patches = string(b)
		} else {
			return 0, formatError(err)
		}
	}

	switch v := call.Sources.(type) {
	case []map[string]any:
		if b, err = json.Marshal(v); err == nil {
			sources = string(b)
		} else {
			return 0, formatError(err)
		}
	}

	if res, err = db.Sql.Exec("insert into `rdioScannerCalls` (`id`, `audio`, `audioName`, `audioType`, `dateTime`, `frequencies`, `frequency`, `patches`, `source`, `sources`, `system`, `talkgroup`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", call.Id, call.Audio, call.AudioName, call.AudioType, call.DateTime, frequencies, call.Frequency, patches, call.Source, sources, call.System, call.Talkgroup); err != nil {
		return 0, formatError(err)
	}

	if id, err = res.LastInsertId(); err == nil {
		return uint(id), nil
	} else {
		return 0, formatError(err)
	}
}

type CallsSearchOptions struct {
	Date                    any `json:"date,omitempty"`
	Group                   any `json:"group,omitempty"`
	Limit                   any `json:"limit,omitempty"`
	Offset                  any `json:"offset,omitempty"`
	Sort                    any `json:"sort,omitempty"`
	System                  any `json:"system,omitempty"`
	Tag                     any `json:"tag,omitempty"`
	Talkgroup               any `json:"talkgroup,omitempty"`
	searchPatchedTalkgroups bool
}

func (searchOptions *CallsSearchOptions) fromMap(m map[string]any) error {
	switch v := m["date"].(type) {
	case string:
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			searchOptions.Date = t
		}
	}

	switch v := m["group"].(type) {
	case string:
		searchOptions.Group = v
	}

	switch v := m["limit"].(type) {
	case float64:
		searchOptions.Limit = uint(v)
	}

	switch v := m["offset"].(type) {
	case float64:
		searchOptions.Offset = uint(v)
	}

	switch v := m["sort"].(type) {
	case float64:
		searchOptions.Sort = int(v)
	}

	switch v := m["system"].(type) {
	case float64:
		searchOptions.System = uint(v)
	}

	switch v := m["tag"].(type) {
	case string:
		searchOptions.Tag = v
	}

	switch v := m["talkgroup"].(type) {
	case float64:
		searchOptions.Talkgroup = uint(v)
	}

	return nil
}

type CallsSearchResult struct {
	Id        uint      `json:"id"`
	DateTime  time.Time `json:"dateTime"`
	System    uint      `json:"system"`
	Talkgroup uint      `json:"talkgroup"`
}

type CallsSearchResults struct {
	Count     uint                `json:"count"`
	DateStart time.Time           `json:"dateStart"`
	DateStop  time.Time           `json:"dateStop"`
	Options   *CallsSearchOptions `json:"options"`
	Results   []CallsSearchResult `json:"results"`
}
