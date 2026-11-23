// Package inventory keeps stock levels and a history of moves: a library
// crate (RFC 0014) that uses pointers inside, and errors as values.
package inventory

import (
	"errors"
	"fmt"
)

// ErrShort is wrapped when a move would take more than is in stock.
var ErrShort = errors.New("inventory: not enough stock")

// Item is one product's stock.
type Item struct {
	SKU   string // the product
	Count int
}

// move is one change, kept in a linked list, newest first.
type move struct {
	sku   string
	delta int
	next  *move
}

func push(head *move, sku string, delta int) *move {
	return &move{sku: sku, delta: delta, next: head}
}

// Apply returns the items after the moves, in order: each move adds delta
// to its item's count, and fails if a count would fall below zero.
func Apply(items []Item, skus []string, deltas []int) ([]Item, error) {
	var history *move
	for i := range skus {
		history = push(history, skus[i], deltas[i])
	}
	var out []Item
	for _, it := range items {
		count := it.Count
		var steps []int
		for m := history; m != nil; m = m.next {
			if m.sku == it.SKU {
				steps = append(steps, m.delta)
			}
		}
		for j := len(steps) - 1; j >= 0; j-- {
			count += steps[j]
			if count < 0 {
				return nil, fmt.Errorf("%s: %w", it.SKU, ErrShort)
			}
		}
		out = append(out, Item{SKU: it.SKU, Count: count})
	}
	return out, nil
}

// Report applies the moves and describes the outcome.
func Report(items []Item, skus []string, deltas []int) string {
	_, err := Apply(items, skus, deltas)
	return describe(err)
}

// describe returns an error's text, or "ok".
func describe(err error) string {
	if err == nil {
		return "ok"
	}
	return "failed: " + err.Error()
}
