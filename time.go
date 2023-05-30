// Copyright 2023 Kevin Conway
// Copyright @ 2017 Atlassian Pty Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rolling

import (
	"sync"
	"time"
)

// TimePolicy is a window Accumulator implementation that uses some
// duration of time to determine the content of the window.
type TimePolicy[T Numeric] struct {
	bucketSize        time.Duration
	bucketSizeNano    int64
	numberOfBuckets   int
	numberOfBuckets64 int64
	window            Window[T]
	lastWindowOffset  int
	lastWindowTime    int64
	lock              *sync.Mutex
}

// NewTimePolicy manages a window with rolling time duratinos.
// The given duration will be used to bucket data within the window. If data
// points are received entire windows aparts then the window will only contain
// a single data point. If one or more durations of the window are missed then
// they are zeroed out to keep the window consistent.
func NewTimePolicy[T Numeric](window Window[T], bucketDuration time.Duration) *TimePolicy[T] {
	return &TimePolicy[T]{
		bucketSize:        bucketDuration,
		bucketSizeNano:    bucketDuration.Nanoseconds(),
		numberOfBuckets:   len(window),
		numberOfBuckets64: int64(len(window)),
		window:            window,
		lock:              &sync.Mutex{},
	}
}

func (w *TimePolicy[T]) resetWindow() {
	for offset := range w.window {
		w.window[offset] = w.window[offset][:0]
	}
}

func (w *TimePolicy[T]) resetBuckets(windowOffset int) {
	var distance = windowOffset - w.lastWindowOffset
	// If the distance between current and last is negative then we've wrapped
	// around the ring. Recalculate the distance.
	if distance < 0 {
		distance = (w.numberOfBuckets - w.lastWindowOffset) + windowOffset
	}
	for counter := 1; counter < distance; counter = counter + 1 {
		var offset = (counter + w.lastWindowOffset) % w.numberOfBuckets
		w.window[offset] = w.window[offset][:0]
	}
}

func (w *TimePolicy[T]) keepConsistent(adjustedTime int64, windowOffset int) {
	// If we've waiting longer than a full window for data then we need to clear
	// the internal state completely.
	if adjustedTime-w.lastWindowTime > w.numberOfBuckets64 {
		w.resetWindow()
	}

	// When one or more buckets are missed we need to zero them out.
	if adjustedTime != w.lastWindowTime && adjustedTime-w.lastWindowTime < w.numberOfBuckets64 {
		w.resetBuckets(windowOffset)
	}
}

func (w *TimePolicy[T]) selectBucket(currentTime time.Time) (int64, int) {
	var adjustedTime = currentTime.UnixNano() / w.bucketSizeNano
	var windowOffset = int(adjustedTime % w.numberOfBuckets64)
	return adjustedTime, windowOffset
}

// AppendWithTimestamp same as Append but with timestamp as parameter
func (w *TimePolicy[T]) AppendWithTimestamp(value T, timestamp time.Time) {
	w.lock.Lock()
	defer w.lock.Unlock()

	var adjustedTime, windowOffset = w.selectBucket(timestamp)
	w.keepConsistent(adjustedTime, windowOffset)
	if w.lastWindowOffset != windowOffset {
		w.window[windowOffset] = []T{value}
	} else {
		w.window[windowOffset] = append(w.window[windowOffset], value)
	}
	w.lastWindowTime = adjustedTime
	w.lastWindowOffset = windowOffset
}

// Append a value to the window using a time bucketing strategy.
func (w *TimePolicy[T]) Append(value T) {
	w.AppendWithTimestamp(value, time.Now())
}

// Reduce the window to a single value using a reduction function.
func (w *TimePolicy[T]) Reduce(f Reduction[T]) T {
	w.lock.Lock()
	defer w.lock.Unlock()

	var adjustedTime, windowOffset = w.selectBucket(time.Now())
	w.keepConsistent(adjustedTime, windowOffset)
	return f(w.window)
}
