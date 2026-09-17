package db

import (
	"os"
	"testing"
)

func TestDeleteSubdomain(t *testing.T) {
	dbFile := "./test_delete_sub.sqlite"
	_ = os.Remove(dbFile)
	defer os.Remove(dbFile)

	database, err := InitDB(dbFile)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	sub := "testdel"
	if err := database.CreateUser("uid-1", sub); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := database.AddRecord(&Record{
		ID:        "rec-1",
		Subdomain: sub,
		Name:      "@",
		Type:      "A",
		Value:     "1.2.3.4",
		TTL:       60,
	}); err != nil {
		t.Fatalf("AddRecord failed: %v", err)
	}

	recs, err := database.GetRecords(sub)
	if err != nil || len(recs) != 1 {
		t.Fatalf("Expected 1 record before delete, got %d (err: %v)", len(recs), err)
	}

	if err := database.DeleteSubdomain(sub); err != nil {
		t.Fatalf("DeleteSubdomain failed: %v", err)
	}

	user, err := database.GetUserBySubdomain(sub)
	if err == nil && user != nil {
		t.Fatalf("Expected user to be deleted, found: %+v", user)
	}

	recsAfter, err := database.GetRecords(sub)
	if err != nil || len(recsAfter) != 0 {
		t.Fatalf("Expected 0 records after delete, got %d", len(recsAfter))
	}
}
