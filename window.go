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

// Window represents a bucketed set of data. It should be used in conjunction
// with a Policy to populate it with data using some windowing policy.
type Window [][]float64

// NewWindow creates a Window with the given number of buckets. The number of
// buckets is meaningful to each Policy. The Policy implementations
// will describe their use of buckets.
func NewWindow(buckets int) Window {
	return make([][]float64, buckets)
}

// NewPreallocatedWindow creates a Window both with the given number of buckets
// and with a preallocated bucket size. This constructor may be used when the
// number of data points per-bucket can be estimated and/or when the desire is
// to allocate a large slice so that allocations do not happen as the Window
// is populated by a Policy.
func NewPreallocatedWindow(buckets int, bucketSize int) Window {
	var w = NewWindow(buckets)
	for offset := range w {
		w[offset] = make([]float64, 0, bucketSize)
	}
	return w
}
