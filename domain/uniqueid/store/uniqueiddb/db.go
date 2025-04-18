// Code generated by godddx, DO AVOID EDIT.
package uniqueiddb

import (
	"github.com/ixugo/goddd/domain/uniqueid"
	"gorm.io/gorm"
)

var _ uniqueid.Storer = DB{}

// DB Related business namespaces
type DB struct {
	db *gorm.DB
}

// NewDB instance object
func NewDB(db *gorm.DB) DB {
	return DB{db: db}
}

// UniqueID Get business instance
func (d DB) UniqueID() uniqueid.UniqueIDStorer {
	return UniqueID(d)
}

// AutoMigrate sync database
func (d DB) AutoMigrate(ok bool) DB {
	if !ok {
		return d
	}
	if err := d.db.AutoMigrate(
		new(uniqueid.UniqueID),
	); err != nil {
		panic(err)
	}
	return d
}
