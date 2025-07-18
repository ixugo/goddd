// Code generated by gowebx, DO AVOID EDIT.
package tokendb

import (
	"github.com/ixugo/goddd/domain/token"
	"gorm.io/gorm"
)

var _ token.Storer = DB{}

// DB Related business namespaces
type DB struct {
	db *gorm.DB
}

// NewDB instance object
func NewDB(db *gorm.DB) DB {
	return DB{db: db}
}

// Token Get business instance
func (d DB) Token() token.TokenStorer {
	return Token(d)
}

// AutoMigrate sync database
func (d DB) AutoMigrate(ok bool) DB {
	if !ok {
		return d
	}
	if err := d.db.AutoMigrate(
		new(token.Token),
	); err != nil {
		panic(err)
	}
	return d
}
