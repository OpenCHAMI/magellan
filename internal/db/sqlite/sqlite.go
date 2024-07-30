package sqlite

import (
	"fmt"

	magellan "github.com/OpenCHAMI/magellan/internal"

	"github.com/jmoiron/sqlx"
)

func CreateProbeResultsIfNotExists(path string) (*sqlx.DB, error) {
	schema := `
	CREATE TABLE IF NOT EXISTS magellan_scanned_ports (
		host TEXT NOT NULL,
		port INTEGER NOT NULL,
		protocol TEXT,
		state INTEGER,
		timestamp TIMESTAMP,
		PRIMARY KEY (host, port)
	);
	`
	// TODO: it may help with debugging to check for file permissions here first
	db, err := sqlx.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed toopen database: %v", err)
	}
	db.MustExec(schema)
	return db, nil
}

func InsertProbeResults(path string, states *[]magellan.ScannedResult) error {
	if states == nil {
		return fmt.Errorf("states == nil")
	}

	// create database if it doesn't already exist
	db, err := CreateProbeResultsIfNotExists(path)
	if err != nil {
		return err
	}

	// insert all probe states into db
	tx := db.MustBegin()
	for _, state := range *states {
		sql := `INSERT OR REPLACE INTO magellan_scanned_ports (host, port, protocol, state, timestamp)
		VALUES (:host, :port, :protocol, :state, :timestamp);`
		_, err := tx.NamedExec(sql, &state)
		if err != nil {
			fmt.Printf("failed toexecute transaction: %v\n", err)
		}
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed tocommit transaction: %v", err)
	}
	return nil
}

func DeleteProbeResults(path string, results *[]magellan.ScannedResult) error {
	if results == nil {
		return fmt.Errorf("no probe results found")
	}
	db, err := sqlx.Open("sqlite3", path)
	if err != nil {
		return fmt.Errorf("failed toopen database: %v", err)
	}
	tx := db.MustBegin()
	for _, state := range *results {
		sql := `DELETE FROM magellan_scanned_ports WHERE host = :host, port = :port;`
		_, err := tx.NamedExec(sql, &state)
		if err != nil {
			fmt.Printf("failed toexecute transaction: %v\n", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed tocommit transaction: %v", err)
	}
	return nil
}

func GetScannedResults(path string) ([]magellan.ScannedResult, error) {
	db, err := sqlx.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed toopen database: %v", err)
	}

	results := []magellan.ScannedResult{}
	err = db.Select(&results, "SELECT * FROM magellan_scanned_ports ORDER BY host ASC, port ASC;")
	if err != nil {
		return nil, fmt.Errorf("failed toretrieve probes: %v", err)
	}
	return results, nil
}
