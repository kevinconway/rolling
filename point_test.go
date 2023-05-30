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
	"fmt"
	"math"
	"testing"
	"time"
)

func TestPointWindow(t *testing.T) {
	var numberOfPoints = 100
	var w = NewWindow(numberOfPoints)
	var p = NewPointPolicy(w)
	for x := 0; x < numberOfPoints; x = x + 1 {
		p.Append(1)
	}
	var final = p.Reduce(func(w Window) float64 {
		var result float64
		for _, bucket := range w {
			for _, p := range bucket {
				result = result + p
			}
		}
		return result
	})
	if final != float64(numberOfPoints) {
		t.Fatal(final)
	}
}

func TestPointWindowDataRace(t *testing.T) {
	var numberOfPoints = 100
	var w = NewWindow(numberOfPoints)
	var p = NewPointPolicy(w)
	var stop = make(chan bool)
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				p.Append(1)
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
				_ = p.Reduce(func(w Window) float64 {
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
	var bucketSizes = []int{1, 10, 100, 1000, 10000}
	var insertions = []int{1, 1000, 10000}
	for _, size := range bucketSizes {
		for _, insertion := range insertions {
			b.Run(fmt.Sprintf("Window Size:%d | Insertions:%d", size, insertion), func(bt *testing.B) {
				var w = NewWindow(size)
				var p = NewPointPolicy(w)
				bt.ResetTimer()
				for n := 0; n < bt.N; n = n + 1 {
					for x := 0; x < insertion; x = x + 1 {
						p.Append(1)
					}
				}
			})
		}
	}
}
