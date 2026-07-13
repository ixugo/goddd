package hook

import "testing"

func TestUseCache(t *testing.T) {
	cacheFn := UseCache(func(i int) (int, error) {
		return i, nil
	})

	for i := range 3 {
		v, ok, _ := cacheFn(i)
		if ok {
			t.Fatal("expect not ok")
		}
		if v != i {
			t.Fatal("expect", i, "got", v)
		}
	}

	for i := range 3 {
		v, ok, _ := cacheFn(i)
		if !ok {
			t.Fatal("expect ok")
		}
		if v != i {
			t.Fatal("expect", i, "got", v)
		}
	}
}
