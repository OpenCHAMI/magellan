package sqlite

import (
	"fmt"
	"strings"

	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/OpenCHAMI/magellan/internal/util"

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

func InsertScannedAssets(path string, assets ...magellan.RemoteAsset) error {
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

func DeleteScannedAssets(path string, assets ...magellan.RemoteAsset) error {
	var (
		db  *sqlx.DB
		tx  *sqlx.Tx
		err error
	)
	if assets == nil {
		return fmt.Errorf("no assets found")
	}
	db, err = sqlx.Open("sqlite3", path)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	tx = db.MustBegin()
	for _, asset := range assets {
		// skip if neither host nor port are specified
		if asset.Host == "" && asset.Port <= 0 {
			continue
		}
		sql := fmt.Sprintf(`DELETE FROM %s`, TABLE_NAME)
		where := []string{}
		if asset.Port > 0 {
			where = append(where, "port=:port")
		}
		if asset.Host != "" {
			where = append(where, "host=:host")
		}
		if len(where) <= 0 {
			continue
		}
		sql += fmt.Sprintf(" WHERE %s;", strings.Join(where, " AND "))
		_, err := tx.NamedExec(sql, &asset)
		if err != nil {
			fmt.Printf("failed to execute DELETE transaction: %v\n", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}
	return nil
}

func GetScannedAssets(path string) ([]magellan.RemoteAsset, error) {
	// check if path exists first to prevent creating the database
	_, exists := util.PathExists(path)
	if !exists {
		return nil, fmt.Errorf("no file found")
	}

	// now check if the file is the SQLite database
	db, err := sqlx.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	results := []magellan.RemoteAsset{}
	err = db.Select(&results, fmt.Sprintf("SELECT * FROM %s ORDER BY host ASC, port ASC;", TABLE_NAME))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve assets: %v", err)
	}
	return results, nil
}
