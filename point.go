// SPDX-FileCopyrightText: © 2023 Kevin Conway
// SPDX-FileCopyrightText: © 2017 Atlassian Pty Ltd
// SPDX-License-Identifier: Apache-2.0

package rolling

import (
	"context"
	"sync"
)

// PointPolicy is a rolling window policy that tracks the last N
// values inserted regardless of insertion time.
type PointPolicy[T Numeric] struct {
	windowSize int
	window     Window[T]
	offset     int
}

// NewPointPolicy generates a Policy that operates on a rolling set of input
// points. The number of points is determined by the size of the given window.
// Each bucket will contain, at most, one data point when the window is full.
//
// Note that this implementation is not safe for concurrent use. Use
// PointPolicyConcurrent if concurrency is required.
func NewPointPolicy[T Numeric](window Window[T]) *PointPolicy[T] {
	var p = &PointPolicy[T]{
		windowSize: len(window),
		window:     window,
	}
	for offset, bucket := range window {
		if len(bucket) < 1 {
			window[offset] = make([]T, 0, 1)
		}
	}
	return p
}

// Append a value to the window.
func (w *PointPolicy[T]) Append(ctx context.Context, value T) {
	if len(w.window[w.offset]) < 1 {
		w.window[w.offset] = append(w.window[w.offset], value)
	}
	w.window[w.offset][0] = value
	w.offset = (w.offset + 1) % w.windowSize
}

// Reduce the window to a single value using a reduction function.
func (w *PointPolicy[T]) Reduce(ctx context.Context, f Reduction[T]) T {
	return f(ctx, w.window)
}

// PointPolicyConcurrent is a concurrent safe wrapper for PointPolicy that uses
// a mutex to serialize calls.
type PointPolicyConcurrent[T Numeric] struct {
	p    *PointPolicy[T]
	lock sync.Locker
}

// NewPointPolicyConcurrent generates a variant of PointPolicy that is safe for
// use in cases of concurrent reading and writing.
func NewPointPolicyConcurrent[T Numeric](window Window[T]) *PointPolicyConcurrent[T] {
	var p = &PointPolicy[T]{
		windowSize: len(window),
		window:     window,
	}
	for offset, bucket := range window {
		if len(bucket) < 1 {
			window[offset] = make([]T, 0, 1)
		}
	}
	return &PointPolicyConcurrent[T]{
		p:    p,
		lock: &sync.Mutex{},
	}
}

func (self *PointPolicyConcurrent[T]) Append(ctx context.Context, value T) {
	self.lock.Lock()
	defer self.lock.Unlock()
	self.p.Append(ctx, value)
}

func (self *PointPolicyConcurrent[T]) Reduce(ctx context.Context, f Reduction[T]) T {
	self.lock.Lock()
	defer self.lock.Unlock()
	return self.p.Reduce(ctx, f)
}
