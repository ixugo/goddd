package uniqueiddb

import (
	"context"
	"testing"

	"github.com/ixugo/goddd/domain/uniqueid"
	"github.com/ixugo/goddd/pkg/orm"
)

func TestUniqueIDGet(t *testing.T) {
	db, mock, err := generateMockDB()
	if err != nil {
		t.Fatal(err)
	}
	userDB := NewUniqueID(db)

	mock.ExpectQuery(`SELECT \* FROM "unique_ids" WHERE id=\$1 (.+) LIMIT \$2`).WithArgs("jack", 1)
	var out uniqueid.UniqueID
	if err := userDB.Get(context.Background(), &out, orm.Where("id=?", "jack")); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal("ExpectationsWereMet err:", err)
	}
}
