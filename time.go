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

// TimePolicy is a rolling window policy that tracks the all values
// inserted over the last N period of time. Each bucket of a window
// represents a duration of time such that the window represents an
// amount of time equal to the sum. For example, 10 buckets in the
// window and a 100ms duration equal a 1s window. This policy rolls
// data out of the window by bucket such that it only contains data
// for the last total window.
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

// NewTimePolicy manages a window with rolling time durations. The given
// duration will be used to bucket data within the window. If data points are
// received entire windows aparts then the window will only contain a single
// data point. If one or more durations of the window are missed then they are
// zeroed out to keep the window consistent.
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

func (w *TimePolicy[T]) keepConsistent(adjustedTime int64, windowOffset int) {
	if adjustedTime-w.lastWindowTime < 0 {
		return
	}
	distance := int(adjustedTime - w.lastWindowTime)
	if distance >= w.numberOfBuckets {
		distance = w.numberOfBuckets
	}
	for x := 1; x <= distance; x = x + 1 {
		offset := (x + w.lastWindowOffset) % w.numberOfBuckets
		w.window[offset] = w.window[offset][:0]
	}
}

func (w *TimePolicy[T]) selectBucket(currentTime time.Time) (int64, int) {
	var adjustedTime = currentTime.UnixNano() / w.bucketSizeNano
	var windowOffset = int(adjustedTime % w.numberOfBuckets64)
	return adjustedTime, windowOffset
}

// AppendWithTimestamp is the same as Append but with timestamp as parameter.
// Note that this method is only for advanced use cases where the clock source
// is external to the system accumulating data. Users of this method must ensure
// that each call is given a timestamp value that is valid for the active window
// of time. Valid values include those that are less than the window's size in
// the past compared to the largest recorded timestamp and those that are equal
// to or greater than the current largest recorded timestamp. Value that are
// older than the window are discarded.
func (w *TimePolicy[T]) AppendWithTimestamp(value T, timestamp time.Time) {
	w.lock.Lock()
	defer w.lock.Unlock()

	var adjustedTime, windowOffset = w.selectBucket(timestamp)
	if adjustedTime-w.lastWindowTime <= -w.numberOfBuckets64 {
		return
	}
	w.keepConsistent(adjustedTime, windowOffset)
	w.window[windowOffset] = append(w.window[windowOffset], value)
	if adjustedTime > w.lastWindowTime {
		w.lastWindowTime = adjustedTime
		w.lastWindowOffset = windowOffset
	}
}

// Append a value to the window using a time bucketing strategy.
func (w *TimePolicy[T]) Append(value T) {
	w.AppendWithTimestamp(value, time.Now())
}

// ReduceWithTimestamp is the same as Reduce but takes the current timestamp
// as a parameter. The timestamp value must be valid according to the same rules
// described in the AppendWithTimestamp method. Invalid timestamp values result
// in a zero value being returned regardless of the reduction function given.
//
// Note that the given timestamp determines only which buckets must be emptied
// and is a mutable operations. The full window of remaining data is always
// given to the reduction method regardless of the timestamp.
func (w *TimePolicy[T]) ReduceWithTimestamp(f Reduction[T], timestamp time.Time) T {
	w.lock.Lock()
	defer w.lock.Unlock()

	var adjustedTime, windowOffset = w.selectBucket(timestamp)
	if adjustedTime-w.lastWindowTime <= -w.numberOfBuckets64 {
		var zero T
		return zero
	}
	w.keepConsistent(adjustedTime, windowOffset)
	if adjustedTime > w.lastWindowTime {
		w.lastWindowTime = adjustedTime
		w.lastWindowOffset = windowOffset
	}
	return f(w.window)
}

// Reduce the window to a single value using a reduction function.
func (w *TimePolicy[T]) Reduce(f Reduction[T]) T {
	return w.ReduceWithTimestamp(f, time.Now())
}
