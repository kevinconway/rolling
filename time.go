// SPDX-FileCopyrightText: © 2023 Kevin Conway
// SPDX-FileCopyrightText: © 2017 Atlassian Pty Ltd
// SPDX-License-Identifier: Apache-2.0

package rolling

import (
	"context"
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
}

// NewTimePolicy manages a window with rolling time durations. The given
// duration will be used to bucket data within the window. If data points are
// received entire windows aparts then the window will only contain a single
// data point. If one or more durations of the window are missed then they are
// zeroed out to keep the window consistent.
//
// Note that this implementation is not safe for concurrent use. Use
// NewTimePolicyConcurrent for a safe implementation.
func NewTimePolicy[T Numeric](window Window[T], bucketDuration time.Duration) *TimePolicy[T] {
	return &TimePolicy[T]{
		bucketSize:        bucketDuration,
		bucketSizeNano:    bucketDuration.Nanoseconds(),
		numberOfBuckets:   len(window),
		numberOfBuckets64: int64(len(window)),
		window:            window,
	}
}

// keepConsistent is called each time the window is modified in order to apply
// the window rolling behavior. This library does not use background goroutines
// to actively manage bucket rotation. Instead, it tracks the most recent, or
// largest, timestamp that has been seen and compares that to the current time
// to determine how much time has passed. This method performs that comparison
// and empties any buckets that have rolled out of the window.
func (self *TimePolicy[T]) keepConsistent(adjustedTime int64, windowOffset int) {
	if adjustedTime-self.lastWindowTime < 0 {
		return
	}
	distance := int(adjustedTime - self.lastWindowTime)
	if distance >= self.numberOfBuckets {
		distance = self.numberOfBuckets
	}
	for x := 1; x <= distance; x = x + 1 {
		offset := (x + self.lastWindowOffset) % self.numberOfBuckets
		self.window[offset] = self.window[offset][:0]
	}
}

// selectBucket returns the current time in units of the bucket size and the
// bucket that would contain that time. For example, if the currentTime
// represents 5.7 seconds after the unix epoch and the bucket size is 1s then
// the first return value of this method would be 5. The bucket is then selected
// with a modulo but this only represents the bucket that would contain the
// current time if it was in the window. This method does not guard against
// future or past times.
func (self *TimePolicy[T]) selectBucket(currentTime time.Time) (int64, int) {
	var adjustedTime = currentTime.UnixNano() / self.bucketSizeNano
	var windowOffset = int(adjustedTime % self.numberOfBuckets64)
	return adjustedTime, windowOffset
}

// AppendWithTimestamp is the same as Append but with timestamp as parameter.
// Note that this method is only for advanced use cases where the clock source
// is external to the system accumulating data. Users of this method must ensure
// that each call is given a timestamp value that is valid for the active window
// of time.
//
// Valid timestamps are technically any value greater than or equal to
// the window's TAIL which is calculated as (HEAD - (BUCKETS*BUCKET_DURATION))
// where HEAD represents the largest timestamp that was previously given to
// AppendWithTimestamp or ReduceWithTimestamp. Values between HEAD and TAIL are
// placed within existing buckets. Values less than TAIL are ignore because
// those timestamps represent a time in the past that is no longer covered by
// the window. Values greater than HEAD permanently move the window forward in
// time.
func (self *TimePolicy[T]) AppendWithTimestamp(ctx context.Context, value T, timestamp time.Time) {
	var adjustedTime, windowOffset = self.selectBucket(timestamp)
	if adjustedTime-self.lastWindowTime <= -self.numberOfBuckets64 {
		return
	}
	self.keepConsistent(adjustedTime, windowOffset)
	self.window[windowOffset] = append(self.window[windowOffset], value)
	if adjustedTime > self.lastWindowTime {
		self.lastWindowTime = adjustedTime
		self.lastWindowOffset = windowOffset
	}
}

// Append a value to the window using a time bucketing strategy.
func (self *TimePolicy[T]) Append(ctx context.Context, value T) {
	self.AppendWithTimestamp(ctx, value, time.Now())
}

// ReduceWithTimestamp is the same as Reduce but takes the current timestamp
// as a parameter. The timestamp value must be valid according to the same rules
// described in the AppendWithTimestamp method. Invalid timestamp values result
// in a zero value being returned regardless of the reduction function or the
// current window contents.
//
// Note that the given timestamp does not necessarily limit the view of the
// window. This is not a "reduce until" method. The given timestamp represents
// the point in time at which the window is being evaluated and it is used to
// roll any expired buckets out of the window before being reduced. This is not
// a read-only method and the effects of using a future time here are the same
// as documented in AppendWithTimestamp.
func (self *TimePolicy[T]) ReduceWithTimestamp(ctx context.Context, f Reduction[T], timestamp time.Time) T {
	var adjustedTime, windowOffset = self.selectBucket(timestamp)
	if adjustedTime-self.lastWindowTime <= -self.numberOfBuckets64 {
		var zero T
		return zero
	}
	self.keepConsistent(adjustedTime, windowOffset)
	if adjustedTime > self.lastWindowTime {
		self.lastWindowTime = adjustedTime
		self.lastWindowOffset = windowOffset
	}
	return f(ctx, self.window)
}

// Reduce the window to a single value using a reduction function.
func (self *TimePolicy[T]) Reduce(ctx context.Context, f Reduction[T]) T {
	return self.ReduceWithTimestamp(ctx, f, time.Now())
}

// TimePolicyConcurrent is a concurrent safe wrapper for TimePolicy that uses a
// mutex to serialize calls.
type TimePolicyConcurrent[T Numeric] struct {
	p    *TimePolicy[T]
	lock sync.Locker
}

// NewTimePolicyConcurrent generates a variant of TimePolicy that is safe for
// concurrent use.
func NewTimePolicyConcurrent[T Numeric](window Window[T], bucketDuration time.Duration) *TimePolicyConcurrent[T] {
	p := NewTimePolicy[T](window, bucketDuration)
	return &TimePolicyConcurrent[T]{
		p:    p,
		lock: &sync.Mutex{},
	}
}

func (self *TimePolicyConcurrent[T]) AppendWithTimestamp(ctx context.Context, value T, timestamp time.Time) {
	self.lock.Lock()
	defer self.lock.Unlock()

	self.p.AppendWithTimestamp(ctx, value, timestamp)
}
func (self *TimePolicyConcurrent[T]) Append(ctx context.Context, value T) {
	self.lock.Lock()
	defer self.lock.Unlock()

	self.p.Append(ctx, value)
}
func (self *TimePolicyConcurrent[T]) ReduceWithTimestamp(ctx context.Context, f Reduction[T], timestamp time.Time) T {
	self.lock.Lock()
	defer self.lock.Unlock()

	return self.p.ReduceWithTimestamp(ctx, f, timestamp)
}
func (self *TimePolicyConcurrent[T]) Reduce(ctx context.Context, f Reduction[T]) T {
	self.lock.Lock()
	defer self.lock.Unlock()

	return self.p.Reduce(ctx, f)
}
