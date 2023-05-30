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

import "sync"

// PointPolicy is a rolling window policy that tracks the last N
// values inserted regardless of insertion time.
type PointPolicy struct {
	windowSize int
	window     Window
	offset     int
	lock       *sync.RWMutex
}

// NewPointPolicy generates a Policy that operates on a rolling set of
// input points. The number of points is determined by the size of the given
// window. Each bucket will contain, at most, one data point when the window
// is full.
func NewPointPolicy(window Window) *PointPolicy {
	var p = &PointPolicy{
		windowSize: len(window),
		window:     window,
		lock:       &sync.RWMutex{},
	}
	for offset, bucket := range window {
		if len(bucket) < 1 {
			window[offset] = make([]float64, 1)
		}
	}
	return p
}

// Append a value to the window.
func (w *PointPolicy) Append(value float64) {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.window[w.offset][0] = value
	w.offset = (w.offset + 1) % w.windowSize
}

// Reduce the window to a single value using a reduction function.
func (w *PointPolicy) Reduce(f func(Window) float64) float64 {
	w.lock.Lock()
	defer w.lock.Unlock()

	return f(w.window)
}
