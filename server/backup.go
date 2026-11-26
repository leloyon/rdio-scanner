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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// backupsKept is how many automatic database backups are kept.
const backupsKept = 3

// backupsToDelete returns the oldest names to remove so that only keep remain.
// Names carry a sortable time stamp, so sorting by name sorts by age.
func backupsToDelete(names []string, keep int) []string {
	sorted := append([]string{}, names...)
	sort.Strings(sorted)
	if len(sorted) <= keep {
		return nil
	}
	return sorted[:len(sorted)-keep]
}

// backupBeforeUpdate copies a SQLite database to a dated file next to it the
// first time a new build opens it, before any database change is made. It does
// nothing for a new database or when the same build opened it last time, keeps
// the newest few backups, and never stops the server from starting.
func (db *Database) backupBeforeUpdate() {
	dbPath := db.Config.GetDbFilePath()

	if _, err := os.Stat(dbPath); err != nil {
		return // new database, nothing to back up
	}

	build := fmt.Sprintf("%s-%s", Version, Edition)
	markerPath := dbPath + ".build"

	if b, err := os.ReadFile(markerPath); err == nil && strings.TrimSpace(string(b)) == build {
		return
	}

	backupPath := fmt.Sprintf("%s.backup-%s-%s", dbPath, time.Now().Format("20060102-150405"), build)

	log.Printf("New build %s: backing up the database to %s", build, filepath.Base(backupPath))

	// VACUUM INTO writes a consistent copy even if the database is in use
	if _, err := db.Sql.Exec(fmt.Sprintf("vacuum into '%s'", strings.ReplaceAll(backupPath, "'", "''"))); err != nil {
		log.Printf("Database backup failed (continuing without it): %v", err)
		return
	}

	if err := os.WriteFile(markerPath, []byte(build), 0644); err != nil {
		log.Printf("Could not record the build for backups: %v", err)
	}

	if matches, err := filepath.Glob(dbPath + ".backup-*"); err == nil {
		for _, old := range backupsToDelete(matches, backupsKept) {
			os.Remove(old)
		}
	}
}
