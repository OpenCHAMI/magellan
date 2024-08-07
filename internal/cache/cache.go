package cache

import "database/sql/driver"

type Cache[T any] interface {
	CreateIfNotExists(path string) (driver.Connector, error)
	Insert(path string, data ...T) error
	Delete(path string, data ...T) error
	Get(path string) ([]T, error)
}
