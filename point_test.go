// SPDX-FileCopyrightText: © 2023 Kevin Conway
// SPDX-FileCopyrightText: © 2017 Atlassian Pty Ltd
// SPDX-License-Identifier: Apache-2.0

package rolling

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"
)

func TestPointWindow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var numberOfPoints = 100
	var w = NewWindow[int](numberOfPoints)
	var p = NewPointPolicy(w)
	for x := 0; x < numberOfPoints; x = x + 1 {
		p.Append(ctx, 1)
	}
	var final = p.Reduce(ctx, func(_ context.Context, w Window[int]) int {
		var result int
		for _, bucket := range w {
			for _, p := range bucket {
				result = result + p
			}
		}
		return result
	})
	if final != numberOfPoints {
		t.Fatal(final)
	}
}

func TestPointWindowDataRace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var numberOfPoints = 100
	var w = NewWindow[float64](numberOfPoints)
	var p = NewPointPolicy(w)
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
							v = math.Mod(v, float64(numberOfPoints))
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

func BenchmarkPointWindow(b *testing.B) {
	ctx := context.Background()
	var bucketSizes = []int{1, 10, 100, 1000, 10000}
	var insertions = []int{1, 1000, 10000}
	for _, size := range bucketSizes {
		for _, insertion := range insertions {
			b.Run(fmt.Sprintf("Window Size:%d | Insertions:%d", size, insertion), func(bt *testing.B) {
				var w = NewWindow[int](size)
				var p = NewPointPolicy(w)
				bt.ResetTimer()
				for n := 0; n < bt.N; n = n + 1 {
					for x := 0; x < insertion; x = x + 1 {
						p.Append(ctx, 1)
					}
				}
			})
		}
	}
}
