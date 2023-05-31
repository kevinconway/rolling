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
	"math"
	"sort"
	"sync"
)

type Reduction[T Numeric] func(ctx context.Context, w Window[T]) T

// MinimumPoints is a wrapper for any other reduction that prevents
// the real reduction from running unless there are a sufficient number
// of data points.
func MinimumPoints[T Numeric](points int, r Reduction[T]) Reduction[T] {
	var zero T
	return func(ctx context.Context, w Window[T]) T {
		if Count(ctx, w) >= T(points) {
			return r(ctx, w)
		}
		return zero
	}
}

// Count returns the number of elements in a window.
func Count[T Numeric](_ context.Context, w Window[T]) T {
	result := 0
	for _, bucket := range w {
		result += len(bucket)
	}
	return T(result)
}

// Sum the values within the window.
func Sum[T Numeric](_ context.Context, w Window[T]) T {
	var result T
	for _, bucket := range w {
		for _, p := range bucket {
			result = result + p
		}
	}
	return result
}

// Avg the values within the window.
func Avg[T Numeric](_ context.Context, w Window[T]) T {
	var result T
	var count T
	for _, bucket := range w {
		for _, p := range bucket {
			result = result + p
			count = count + 1
		}
	}
	if count == 0 {
		return 0
	}
	return result / count
}

// Min the values within the window.
func Min[T Numeric](_ context.Context, w Window[T]) T {
	var result T
	var started bool
	for _, bucket := range w {
		for _, p := range bucket {
			if !started {
				result = p
				started = true
				continue
			}
			if p < result {
				result = p
			}
		}
	}
	return result
}

// Max the values within the window.
func Max[T Numeric](_ context.Context, w Window[T]) T {
	var result T
	var started bool
	for _, bucket := range w {
		for _, p := range bucket {
			if !started {
				result = p
				started = true
				continue
			}
			if p > result {
				result = p
			}
		}
	}
	return result
}

// Percentile returns an aggregating function that computes the
// given percentile calculation for a window.
func Percentile[T Numeric](perc float64) func(_ context.Context, w Window[T]) T {
	var zero T
	var values []T
	var lock = &sync.Mutex{}
	return func(_ context.Context, w Window[T]) T {
		lock.Lock()
		defer lock.Unlock()

		values = values[:0]
		for _, bucket := range w {
			values = append(values, bucket...)
		}
		if len(values) < 1 {
			return zero
		}
		sort.SliceStable(values, func(i, j int) bool {
			return values[i] < values[j]
		})
		var position = (float64(len(values))*(perc/100) + .5) - 1
		var k = int(math.Floor(position))
		var f = math.Mod(position, 1)
		if f == 0.0 {
			return values[k]
		}
		var plusOne = k + 1
		if plusOne > len(values)-1 {
			plusOne = k
		}
		return T(((1 - f) * float64(values[k])) + (f * float64(values[plusOne])))
	}
}

// FastPercentile implements the pSquare percentile estimation
// algorithm for calculating percentiles from streams of data
// using fixed memory allocations.
func FastPercentile[T Numeric](perc float64) func(_ context.Context, w Window[T]) T {
	perc = perc / 100.0
	return func(_ context.Context, w Window[T]) T {
		var initalObservations = make([]float64, 0, 5)
		var q [5]float64
		var n [5]int
		var nPrime [5]float64
		var dnPrime [5]float64
		var observations uint64
		for _, bucket := range w {
			for _, vT := range bucket {
				v := float64(vT)
				observations = observations + 1
				// Record first five observations
				if observations < 6 {
					initalObservations = append(initalObservations, v)
					continue
				}
				// Before proceeding beyond the first five, process them.
				if observations == 6 {
					bubbleSort(initalObservations)
					for offset := range q {
						q[offset] = initalObservations[offset]
						n[offset] = offset
					}
					nPrime[0] = 0
					nPrime[1] = 2 * perc
					nPrime[2] = 4 * perc
					nPrime[3] = 2 + 2*perc
					nPrime[4] = 4
					dnPrime[0] = 0
					dnPrime[1] = perc / 2
					dnPrime[2] = perc
					dnPrime[3] = (1 + perc) / 2
					dnPrime[4] = 1
				}
				var k int // k is the target cell to increment
				switch {
				case v < q[0]:
					q[0] = v
					k = 0
				case q[0] <= v && v < q[1]:
					k = 0
				case q[1] <= v && v < q[2]:
					k = 1
				case q[2] <= v && v < q[3]:
					k = 2
				case q[3] <= v && v <= q[4]:
					k = 3
				case v > q[4]:
					q[4] = v
					k = 3
				}
				for x := k + 1; x < 5; x = x + 1 {
					n[x] = n[x] + 1
				}
				nPrime[0] = nPrime[0] + dnPrime[0]
				nPrime[1] = nPrime[1] + dnPrime[1]
				nPrime[2] = nPrime[2] + dnPrime[2]
				nPrime[3] = nPrime[3] + dnPrime[3]
				nPrime[4] = nPrime[4] + dnPrime[4]
				for x := 1; x < 4; x = x + 1 {
					var d = nPrime[x] - float64(n[x])
					if (d >= 1 && (n[x+1]-n[x]) > 1) ||
						(d <= -1 && (n[x-1]-n[x]) < -1) {
						var s = sign(d)
						var si = int(s)
						var nx = float64(n[x])
						var nxPlusOne = float64(n[x+1])
						var nxMinusOne = float64(n[x-1])
						var qx = q[x]
						var qxPlusOne = q[x+1]
						var qxMinusOne = q[x-1]
						var parab = q[x] + (s/(nxPlusOne-nxMinusOne))*((nx-nxMinusOne+s)*(qxPlusOne-qx)/(nxPlusOne-nx)+(nxPlusOne-nx-s)*(qx-qxMinusOne)/(nx-nxMinusOne))
						if qxMinusOne < parab && parab < qxPlusOne {
							q[x] = parab
						} else {
							q[x] = q[x] + s*((q[x+si]-q[x])/float64(n[x+si]-n[x]))
						}
						n[x] = n[x] + si
					}
				}

			}
		}
		var zero T
		if observations < 1 {
			return zero
		}
		// If we have less than five values then degenerate into a max function.
		// This is a reasonable value for data sets this small.
		if observations < 5 {
			bubbleSort(initalObservations)
			return T(initalObservations[len(initalObservations)-1])
		}
		return T(q[2])
	}
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// using bubblesort because we're only working with datasets of 5 or fewer
// elements.
func bubbleSort(s []float64) {
	for range s {
		for x := 0; x < len(s)-1; x = x + 1 {
			if s[x] > s[x+1] {
				s[x], s[x+1] = s[x+1], s[x]
			}
		}
	}
}
