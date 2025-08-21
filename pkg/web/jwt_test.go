package web

import (
	"fmt"
	"testing"
	"time"
)

func TestJWT(t *testing.T) {
	const secret = "test_secret_key"

	data := NewClaimsData().SetLevel(1)
	token, err := NewToken(data, secret, WithExpires(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	cli, err := ParseToken(token, secret)
	v := cli.Data[KeyLevel].(float64)
	if v != 1 {
		t.Fatal("level not equal")
	}

	if err := cli.Valid(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	if err := cli.Valid(); err == nil {
		t.Fatal("valid faild")
	}
}

func TestClaimsData(t *testing.T) {
	data := NewClaimsData()
	data.SetUserID(123)

	if data[KeyUserID] != 123 {
		t.Errorf("SetUserID failed")
	}

	for i := range 100000 {
		data.Set(fmt.Sprintf("key%d", i), i)
	}

	if len(data) != 100001 {
		t.Errorf("Set failed")
	}
}
