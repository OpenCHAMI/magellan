package sqlite

import (
	"fmt"

	magellan "davidallendj/magellan/internal"

	"github.com/jmoiron/sqlx"
)

func InsertProbeResults(path string, states *[]magellan.BMCProbeResult) error {
	if states == nil {
		return fmt.Errorf("states == nil")
	}

	// create database if it doesn't already exist
	schema := `
	CREATE TABLE IF NOT EXISTS magellan_scanned_ports (
		host TEXT PRIMARY KEY NOT NULL,
		port INTEGER,
		protocol TEXT,
		state INTEGER
	);
	`
	db, err := sqlx.Open("sqlite3", path)
	if err != nil {
		return fmt.Errorf("could not open database: %v", err)
	}
	db.MustExec(schema)

	// insert all probe states into db
	tx := db.MustBegin()
	for _, state := range *states {
		sql := `INSERT OR REPLACE INTO magellan_scanned_ports (host, port, protocol, state) 
		VALUES (:host, :port, :protocol, :state);`
		_, err := tx.NamedExec(sql, &state)
		if err != nil {
			fmt.Printf("could not execute transaction: %v\n", err)
		}
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("could not commit transaction: %v", err)
	}
	return nil
}

func GetProbeResults(path string) ([]magellan.BMCProbeResult, error) {
	db, err := sqlx.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %v", err)
	}

	results := []magellan.BMCProbeResult{}
	err = db.Select(&results, "SELECT * FROM magellan_scanned_ports ORDER BY host ASC")
	if err != nil {
		return nil, fmt.Errorf("could not retrieve probes: %v", err)
	}
	return results, nil
}