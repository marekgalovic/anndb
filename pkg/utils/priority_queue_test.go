package utils

import (
    "testing";

    "github.com/stretchr/testify/assert";
)

func TestMinPriorityQueue(t *testing.T) {
    q := NewMinPriorityQueue()

    q.Push(NewPriorityQueueItem(3, "foo"))
    q.Push(NewPriorityQueueItem(1, "bar"))
    q.Push(NewPriorityQueueItem(2, "bag"))

    assert.Equal(t, "bar", q.Peek().Value())
    assert.Equal(t, "bar", q.Peek().Value())

    assert.Equal(t, "bar", q.Pop().Value())
    assert.Equal(t, "bag", q.Pop().Value())
    assert.Equal(t, "foo", q.Pop().Value())
    assert.Equal(t, 0, q.Len())
}

func TestMaxPriorityQueue(t *testing.T) {
    q := NewMaxPriorityQueue()

    q.Push(NewPriorityQueueItem(1, "bar"))
    q.Push(NewPriorityQueueItem(3, "foo"))
    q.Push(NewPriorityQueueItem(2, "bag"))

    assert.Equal(t, "foo", q.Peek().Value())
    assert.Equal(t, "foo", q.Peek().Value())

    assert.Equal(t, "foo", q.Pop().Value())
    assert.Equal(t, "bag", q.Pop().Value())
    assert.Equal(t, "bar", q.Pop().Value())
    assert.Equal(t, 0, q.Len())
}

func TestPriorityQueueReverse(t *testing.T) {
    q := NewMaxPriorityQueue()

    q.Push(NewPriorityQueueItem(3, "foo"))
    q.Push(NewPriorityQueueItem(1, "bar"))
    q.Push(NewPriorityQueueItem(2, "bag"))

    assert.Equal(t, 3, q.Len())
    assert.Equal(t, "foo", q.Peek().Value())

    rq := q.Reverse()

    assert.Equal(t, 3, rq.Len())
    assert.Equal(t, "bar", rq.Peek().Value())

    assert.Equal(t, "bar", rq.Pop().Value())
    assert.Equal(t, "bag", rq.Pop().Value())
    assert.Equal(t, "foo", rq.Pop().Value())
    assert.Equal(t, 0, rq.Len())

    assert.Equal(t, 3, q.Len())
    assert.Equal(t, "foo", q.Peek().Value())
}

func TestPriorityQueueToSlice(t *testing.T) {
    q := NewMaxPriorityQueue()

    q.Push(NewPriorityQueueItem(3, "foo"))
    q.Push(NewPriorityQueueItem(1, "bar"))
    q.Push(NewPriorityQueueItem(2, "bag"))

    s := q.ToSlice()

    assert.Equal(t, 3, len(s))
}

func TestPriorityQueueValues(t *testing.T) {
    q := NewMaxPriorityQueue()

    q.Push(NewPriorityQueueItem(3, "foo"))
    q.Push(NewPriorityQueueItem(1, "bar"))
    q.Push(NewPriorityQueueItem(2, "bag"))

    values := q.Values()

    assert.Equal(t, 3, len(values))
    assert.Equal(t, 3, q.Len())

    valuesSet := NewSet(values...)
    assert.True(t, valuesSet.Contains("foo"))
    assert.True(t, valuesSet.Contains("bar"))
    assert.True(t, valuesSet.Contains("bag"))
}