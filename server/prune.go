package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// v4: talkgroup maintenance. Only talkgroups that were never named (label is
// still just the numeric ID and name is empty or the same) can be pruned.
// Call recordings are never deleted.

func talkgroupIsUnnamed(tg *Talkgroup) bool {
	id := strconv.Itoa(int(tg.Id))
	return tg.Label == id && (tg.Name == "" || tg.Name == id)
}

func talkgroupIsPhantom(tg *Talkgroup) bool {
	return talkgroupIsUnnamed(tg) && (tg.Id < 100 || tg.Id > 65535)
}

func (admin *Admin) TalkgroupsPruneHandler(w http.ResponseWriter, r *http.Request) {
	if !admin.ValidateToken(admin.GetAuthorization(r)) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	req := struct {
		Mode    string `json:"mode"`
		Confirm bool   `json:"confirm"`
	}{}
	if json.NewDecoder(r.Body).Decode(&req) != nil || (req.Mode != "phantom" && req.Mode != "unnamed") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	match := talkgroupIsUnnamed
	if req.Mode == "phantom" {
		match = talkgroupIsPhantom
	}

	c := admin.Controller
	type item struct {
		System      uint   `json:"system"`
		SystemLabel string `json:"systemLabel"`
		Talkgroup   uint   `json:"talkgroup"`
	}
	found := []item{}
	for _, sys := range c.Systems.List {
		keep := []*Talkgroup{}
		for _, tg := range sys.Talkgroups.List {
			if match(tg) {
				found = append(found, item{sys.Id, sys.Label, tg.Id})
			} else {
				keep = append(keep, tg)
			}
		}
		if req.Confirm {
			sys.Talkgroups.List = keep
		}
	}

	if req.Confirm && len(found) > 0 {
		if err := c.Systems.Write(c.Database); err != nil {
			c.Logs.LogEvent(LogLevelError, "talkgroups.prune: "+err.Error())
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}
		c.Systems.Read(c.Database)
		c.Logs.LogEvent(LogLevelInfo, "talkgroups.prune: removed "+strconv.Itoa(len(found))+" "+req.Mode+" talkgroups (recordings kept)")
		c.EmitConfig()
		admin.BroadcastConfig()
	}

	b, _ := json.Marshal(map[string]any{"mode": req.Mode, "confirmed": req.Confirm, "count": len(found), "talkgroups": found})
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}
