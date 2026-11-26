package main

import "testing"

func TestBackupsToDelete(t *testing.T) {
	names := []string{"db.backup-20261128-010000-x", "db.backup-20261126-010000-x", "db.backup-20261127-010000-x", "db.backup-20261129-010000-x"}
	got := backupsToDelete(names, 3)
	if len(got) != 1 || got[0] != "db.backup-20261126-010000-x" {
		t.Fatal(got)
	}
	if backupsToDelete(names[:2], 3) != nil {
		t.Fatal("nothing to delete")
	}
}
