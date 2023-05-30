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
	"context"
	"fmt"
	"math"
	"testing"
	"time"
)

func TestTimeWindow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var bucketSize = time.Millisecond * 100
	var numberBuckets = 10
	var w = NewWindow[int](numberBuckets)
	var p = NewTimePolicy(w, bucketSize)
	for x := 0; x < numberBuckets; x = x + 1 {
		p.Append(ctx, 1)
		if x != numberBuckets-1 {
			time.Sleep(bucketSize)
		}
	}
	var final = p.Reduce(ctx, func(_ context.Context, w Window[int]) int {
		var result int
		for _, bucket := range w {
			for _, point := range bucket {
				result = result + point
			}
		}
		return result
	})
	if final != numberBuckets {
		t.Fatalf("expected %d values but got %d", numberBuckets, final)
	}
	time.Sleep(bucketSize) // Wait to ensure the window has gone at least one full duration
	for x := 0; x < numberBuckets; x = x + 1 {
		p.Append(ctx, 2)
		if x != numberBuckets-1 {
			time.Sleep(bucketSize)
		}
	}

	final = p.Reduce(ctx, func(_ context.Context, w Window[int]) int {
		var result int
		for _, bucket := range w {
			for _, point := range bucket {
				result = result + point
			}
		}
		return result
	})
	if final != 2*numberBuckets {
		t.Fatalf("got %d but expected %d", final, 2*numberBuckets)
	}
}

func TestTimeWindowSelectBucket(t *testing.T) {
	t.Parallel()

	var bucketSize = time.Millisecond * 50
	var numberBuckets = 10
	var w = NewWindow[int](numberBuckets)
	var p = NewTimePolicy(w, bucketSize)
	var target = time.Unix(0, 0)
	var adjustedTime, bucket = p.selectBucket(target)
	if bucket != 0 {
		t.Fatalf("expected bucket 0 but got %d %v", bucket, adjustedTime)
	}
	target = time.Unix(0, int64(50*time.Millisecond))
	_, bucket = p.selectBucket(target)
	if bucket != 1 {
		t.Fatalf("expected bucket 1 but got %d %v", bucket, target)
	}
	target = time.Unix(0, int64(50*time.Millisecond)*10)
	_, bucket = p.selectBucket(target)
	if bucket != 0 {
		t.Fatalf("expected bucket 10 but got %d %v", bucket, target)
	}
	target = time.Unix(0, int64(50*time.Millisecond)*11)
	_, bucket = p.selectBucket(target)
	if bucket != 1 {
		t.Fatalf("expected bucket 0 but got %d %v", bucket, target)
	}
}

func TestTimeWindowConsistency(t *testing.T) {
	t.Parallel()

	var bucketSize = time.Millisecond * 50
	var numberBuckets = 10
	var w = NewWindow[int](numberBuckets)
	var p = NewTimePolicy(w, bucketSize)
	for offset := range p.window {
		p.window[offset] = append(p.window[offset], 1)
	}
	var start = time.Now()
	p.lastWindowTime, p.lastWindowOffset = p.selectBucket(start)
	var sentinelOffset = p.lastWindowOffset
	var target = start
	var adjustedTime, bucket = p.selectBucket(target)
	p.keepConsistent(adjustedTime, bucket)
	if len(p.window[sentinelOffset]) != 1 {
		t.Fatalf("data loss while adjusting internal state %+v", p.window)
	}
	target = start.Add(bucketSize)
	adjustedTime, bucket = p.selectBucket(target)
	p.keepConsistent(adjustedTime, bucket)
	if len(p.window[sentinelOffset]) != 1 {
		t.Fatalf("data loss while adjusting internal state %+v", p.window)
	}
	target = start.Add(5 * bucketSize)
	adjustedTime, bucket = p.selectBucket(target)
	p.keepConsistent(adjustedTime, bucket)
	if len(p.window[sentinelOffset]) != 1 {
		t.Fatalf("data loss while adjusting internal state %+v", p.window)
	}
	for x := 1; x < 5; x = x + 1 {
		y := (sentinelOffset + x) % numberBuckets
		if len(p.window[y]) != 0 {
			t.Fatalf("internal state not kept consistent during time gap %+v", p.window)
		}
	}
}

func TestTimeWindowPastValuesOutOfRange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var bucketSize = time.Millisecond * 50
	var numberBuckets = 5
	var w = NewWindow[int](numberBuckets)
	var p = NewTimePolicy(w, bucketSize)

	var start = time.Now()
	p.AppendWithTimestamp(ctx, 1, start)
	p.AppendWithTimestamp(ctx, 1, start.Add(bucketSize))
	p.AppendWithTimestamp(ctx, 1, start.Add(bucketSize*2))
	p.AppendWithTimestamp(ctx, 1, start.Add(bucketSize*3))
	p.AppendWithTimestamp(ctx, 1, start.Add(bucketSize*4))

	v := p.ReduceWithTimestamp(ctx, Sum[int], start.Add(bucketSize*4))
	if v != 5 {
		t.Fatalf("expected %d but got %d: %+v", 5, v, p.window)
	}
	p.AppendWithTimestamp(ctx, 1, start.Add(-bucketSize))
	v = p.ReduceWithTimestamp(ctx, Sum[int], start.Add(bucketSize*4))
	if v != 5 {
		t.Fatalf("expected %d but got %d: %+v", 5, v, p.window)
	}
	v = p.ReduceWithTimestamp(ctx, Sum[int], start.Add(-bucketSize))
	if v != 0 {
		t.Fatalf("expected %d but got %d: %+v", 0, v, p.window)
	}
}

func TestTimeWindowPastValuesWithinRange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var bucketSize = time.Millisecond * 50
	var numberBuckets = 5
	var w = NewWindow[int](numberBuckets)
	var p = NewTimePolicy(w, bucketSize)

	var start = time.Now()
	p.AppendWithTimestamp(ctx, 1, start)
	p.AppendWithTimestamp(ctx, 1, start.Add(bucketSize))
	p.AppendWithTimestamp(ctx, 1, start.Add(bucketSize*2))
	p.AppendWithTimestamp(ctx, 1, start.Add(bucketSize*3))
	p.AppendWithTimestamp(ctx, 1, start.Add(bucketSize*4))

	v := p.ReduceWithTimestamp(ctx, Sum[int], start.Add(bucketSize*4))
	if v != 5 {
		t.Fatalf("expected %d but got %d: %+v", 5, v, p.window)
	}
	p.AppendWithTimestamp(ctx, 1, start.Add(bucketSize*2))
	v = p.ReduceWithTimestamp(ctx, Sum[int], start.Add(bucketSize*4))
	if v != 6 {
		t.Fatalf("expected %d but got %d: %+v", 6, v, p.window)
	}
	v = p.ReduceWithTimestamp(ctx, Sum[int], start.Add(2*bucketSize))
	if v != 6 {
		t.Fatalf("expected %d but got %d: %+v", 6, v, p.window)
	}
}

func TestTimeWindowDataRace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var bucketSize = time.Millisecond
	var numberBuckets = 1000
	var w = NewWindow[float64](numberBuckets)
	var p = NewTimePolicy(w, bucketSize)
	var stop = make(chan bool)
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				p.Append(ctx, 1)
				time.Sleep(time.Millisecond)
			}
		}
	}()
	go func() {
		var v float64
		for {
			select {
			case <-stop:
				return
			default:
				_ = p.Reduce(ctx, func(_ context.Context, w Window[float64]) float64 {
					for _, bucket := range w {
						for _, p := range bucket {
							v = v + p
							v = math.Mod(v, float64(numberBuckets))
						}
					}
					return 0
				})
			}
		}
	}()
	time.Sleep(time.Second)
	close(stop)
}

type timeWindowOptions struct {
	name          string
	bucketSize    time.Duration
	numberBuckets int
	insertions    int
}

func BenchmarkTimeWindow(b *testing.B) {
	var durations = []time.Duration{time.Millisecond}
	var bucketSizes = []int{1, 10, 100, 1000}
	var insertions = []int{1, 1000, 10000}
	var options = make([]timeWindowOptions, 0, len(durations)*len(bucketSizes)*len(insertions))
	ctx := context.Background()
	for _, d := range durations {
		for _, s := range bucketSizes {
			for _, i := range insertions {
				options = append(
					options,
					timeWindowOptions{
						name:          fmt.Sprintf("Duration:%v | Buckets:%d | Insertions:%d", d, s, i),
						bucketSize:    d,
						numberBuckets: s,
						insertions:    i,
					},
				)
			}
		}
	}
	b.ResetTimer()
	for _, option := range options {
		b.Run(option.name, func(bt *testing.B) {
			var w = NewWindow[int](option.numberBuckets)
			var p = NewTimePolicy(w, option.bucketSize)
			var start = time.Now()
			bt.ResetTimer()
			for n := 0; n < bt.N; n = n + 1 {
				for x := 0; x < option.insertions; x = x + 1 {
					p.AppendWithTimestamp(ctx, 1, start.Add(time.Duration(x)*option.bucketSize))
				}
			}
		})
	}
}
