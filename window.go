// SPDX-FileCopyrightText: © 2023 Kevin Conway
// SPDX-FileCopyrightText: © 2017 Atlassian Pty Ltd
// SPDX-License-Identifier: Apache-2.0

package rolling

import "golang.org/x/exp/constraints"

type Numeric interface {
	constraints.Integer | constraints.Float
}

// Window represents a bucketed set of data. It should be used in conjunction
// with a Policy to populate it with data using some windowing policy.
type Window[T Numeric] [][]T

// NewWindow creates a Window with the given number of buckets. The number of
// buckets is meaningful to each Policy. The Policy implementations
// will describe their use of buckets.
func NewWindow[T Numeric](buckets int) Window[T] {
	return make([][]T, buckets)
}

// NewPreallocatedWindow creates a Window both with the given number of buckets
// and with a preallocated bucket size. This constructor may be used when the
// number of data points per-bucket can be estimated and/or when the desire is
// to allocate a large slice so that allocations do not happen as the Window
// is populated by a Policy.
func NewPreallocatedWindow[T Numeric](buckets int, bucketSize int) Window[T] {
	var w = NewWindow[T](buckets)
	for offset := range w {
		w[offset] = make([]T, 0, bucketSize)
	}
	return w
}
