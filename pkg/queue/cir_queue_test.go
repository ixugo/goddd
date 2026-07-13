package queue

import (
	"testing"
)

func TestPush(t *testing.T) {
	q := NewCirQueue[int](5)

	for i := 1; i <= 100; i++ {
		q.Push(i)
	}
	if q.Size() != 5 {
		t.Errorf("Size() = %d, want %d", q.Size(), 5)
	}
	t.Log(q.Range())
}
