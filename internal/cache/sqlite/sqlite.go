package sqlite

import (
	"fmt"

	magellan "github.com/OpenCHAMI/magellan/internal"

	"github.com/jmoiron/sqlx"
)

const TABLE_NAME = "magellan_scanned_assets"

func CreateScannedAssetIfNotExists(path string) (*sqlx.DB, error) {
	schema := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
		host 		TEXT NOT NULL,
		port 		INTEGER NOT NULL,
		protocol 	TEXT,
		state 		INTEGER,
		timestamp 	TIMESTAMP,
		PRIMARY KEY (host, port)
	);
	`, TABLE_NAME)
	// TODO: it may help with debugging to check for file permissions here first
	db, err := sqlx.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	db.MustExec(schema)
	return db, nil
}

func InsertScannedAssets(path string, assets ...magellan.ScannedAsset) error {
	if assets == nil {
		return fmt.Errorf("states == nil")
	}

	// create database if it doesn't already exist
	db, err := CreateScannedAssetIfNotExists(path)
	if err != nil {
		return err
	}

	// insert all probe states into db
	tx := db.MustBegin()
	for _, state := range assets {
		sql := fmt.Sprintf(`INSERT OR REPLACE INTO %s (host, port, protocol, state, timestamp)
		VALUES (:host, :port, :protocol, :state, :timestamp);`, TABLE_NAME)
		_, err := tx.NamedExec(sql, &state)
		if err != nil {
			fmt.Printf("failed to execute transaction: %v\n", err)
		}
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}
	return nil
}

func DeleteScannedAssets(path string, results ...magellan.ScannedAsset) error {
	if results == nil {
		return fmt.Errorf("no assets found")
	}
	db, err := sqlx.Open("sqlite3", path)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	tx := db.MustBegin()
	for _, state := range results {
		sql := fmt.Sprintf(`DELETE FROM %s WHERE host = :host, port = :port;`, TABLE_NAME)
		_, err := tx.NamedExec(sql, &state)
		if err != nil {
			fmt.Printf("failed to execute transaction: %v\n", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}
	return nil
}

func GetScannedAssets(path string) ([]magellan.ScannedAsset, error) {
	db, err := sqlx.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	results := []magellan.ScannedAsset{}
	err = db.Select(&results, fmt.Sprintf("SELECT * FROM %s ORDER BY host ASC, port ASC;", TABLE_NAME))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve assets: %v", err)
	}
	return results, nil
}
