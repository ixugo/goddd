package web

import (
	"fmt"
	"testing"
)

func TestJWT(t *testing.T) {
	const secret = "test_secret_key"
	// token, _ := NewToken(1, 0, "aabc", secret, time.Second*10)
	// c, err := ParseToken(token, secret)
	// require.NoError(t, err)
	// require.NoError(t, c.Valid())

	// oldTokenStr := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJVSUQiOjEsImlzcyI6Inh4QGdvbGFuZy5zcGFjZSIsImV4cCI6MTY3Njg2MDQ2NywiaWF0IjoxNjc2ODYwNDU3fQ.ACcit_wskXj_Vo5foBonO1oMNPYVQcgIKL81MA7LGHg"
	// _, err = ParseToken(oldTokenStr, secret)
	// require.NotNil(t, err)

	// oldTokenStr = "eyJhbGciOiJIUzI1NVCJ9.ey5zcGFjDU3fQ.ACcit_wskXj_Vo5foBonA7LGHg"
	// _, err = ParseToken(oldTokenStr, secret)
	// require.NotNil(t, err)
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
