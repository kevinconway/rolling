# rolling

**A rolling/sliding window implementation for Go.**

- [rolling](#rolling)
  - [Usage](#usage)
    - [Point Window](#point-window)
    - [Time Window](#time-window)
      - [Time Window With Manual Timestamp Selection](#time-window-with-manual-timestamp-selection)
      - [Out Of Order Timestamps](#out-of-order-timestamps)
    - [Aggregating Windows](#aggregating-windows)
      - [Custom Aggregations](#custom-aggregations)
  - [Performance Considerations](#performance-considerations)
  - [Contributors](#contributors)
  - [License](#license)

## Usage

### Point Window

Rolling point windows record the last `N` data points.

```golang
var p = rolling.NewPointPolicy(rolling.NewWindow[int](5))

for x := 0; x < 5; x = x + 1 {
  p.Append(x)
}
p.Reduce(func(w Window[int]) int {
  fmt.Println(w) // [ [0] [1] [2] [3] [4] ]
  return 0
})
w.Append(5)
p.Reduce(func(w Window[int]) int {
  fmt.Println(w) // [ [5] [1] [2] [3] [4] ]
  return 0
})
w.Append(6)
p.Reduce(func(w Window[int]) int {
  fmt.Println(w) // [ [5] [6] [2] [3] [4] ]
  return 0
})
```

The above creates a window that always contains 5 data points and then fills
it with the values 0 - 4. When the next value is appended it will overwrite
the first value. The window continuously overwrites the oldest value with the
latest to preserve the specified value count. This type of window is useful
for collecting data that have a known interval on which they are captured or
for tracking data where time is not a factor.

### Time Window

Rolling time windows represent all data recorded within the past `T` amount of
time.

```golang
var p = rolling.NewTimeWindow(rolling.NewWindow[int](3000), time.Millisecond)
var start = time.Now()
for range time.Tick(time.Millisecond) {
  if time.Since(start) > 3*time.Second {
    break
  }
  p.Append(1)
}
```

The above creates a time window that contains 3,000 buckets where each bucket
contains, at most, 1ms of recorded data. The subsequent loop populates each
bucket with exactly one measure (the value 1) and stops when the window is full.
As time progresses, the oldest values will be removed such that if the above
code performed a `time.Sleep(3*time.Second)` then the window would be empty
again.

The choice of bucket size depends on the frequency with which data are expected
to be recorded. On each increment of time equal to the given duration the window
will expire one bucket and purge the collected values. The smaller the bucket
duration then the less data are lost when a bucket expires.

This type of bucket is most useful for collecting real-time values such as
request rates, error rates, and latencies of operations.

#### Time Window With Manual Timestamp Selection

The time based rolling window defaults to using `time.Now()` for all timestamp
generation but it is possible to provide your own timestamps using the
`AppendWithTimestamp` and `ReduceWithTimestamp` variants:
```golang
var p = rolling.NewTimeWindow(rolling.NewWindow[int](3000), time.Millisecond)
var start = time.Now()
for range time.Tick(time.Millisecond) {
  if time.Since(start) > 3*time.Second {
    break
  }
  p.AppendWithTimestamp(1, start.Add(time.Millisecond))
}
```

Using custom timestamps is an advanced use case and requires careful usage. The
target use case for this is when populating the window with a series of data
that provides its own timestamps. An example of this is processing a strictly
ordered event stream where each event has an embedded timestamp. If you use the
`WithTimestamp` variants then you should *not* also use the methods that
generate default timestamps. The reason for this is that the time based policy
is driven by the largest given timestamp such that the general expectation is
for timestamps to only increase in value. Mixing custom and default timestamps
can result in unexpected behavior or a corrupt internal state of the policy.

#### Out Of Order Timestamps

The risk of feeding out of order timestamps to a time based policy may be
greater when using custom timestamps but there is still a risk when using the
default timestamp generation. The current implementation leverages wall clock
values to determine passage of time and when routing values to window buckets.
There are multiple ways for wall clock time to run backwards depending on system
configuration and load on the system. Here's how the time policy handles
different ordering conditions:

- If the timestamp is earlier than CURRENT_LARGEST - WINDOW_SIZE then it is
  discarded
- If the timestamp is between CURRENT_LARGEST and CURRENT_LARGEST - WINDOW_SIZE
  then it is added into an existing bucket
- If the timestamp is later than CURRENT_LARGEST then it becomes the new
  CURRENT_LARGEST and the window is modified to reflect this new window boundary

If time runs backwards substantially then it is likely that all data points will
be discarded until the timestamp values return to the range within the window.
If time runs forwards substantially then it will both discard all current values
in the window and set a new anchor point. The window never adjusts backwards so
a future timestamp effectively invalidates the window until the source of
timestamps begins producing values within the window established by the future
timestamp.

While these timing conditions are possible, a substantial forwards or backwards
move in wall clock time is uncommon and would likely have other negative effects
on a system beyond this library.

### Aggregating Windows

Each policy exposes a `Reduce(func(ctx context.Context, w Window[T]) T) T`
method that can be used to aggregate the data stored within the window. The
method takes in a function that can compute the contents of the `Window` into a
single value. For convenience, this package provides some common reductions:

```golang
fmt.Println(p.Reduce(ctx, rolling.Count[int]))
fmt.Println(p.Reduce(ctx, rolling.Avg[int]))
fmt.Println(p.Reduce(ctx, rolling.Min[int]))
fmt.Println(p.Reduce(ctx, rolling.Max[int]))
fmt.Println(p.Reduce(ctx, rolling.Sum[int]))
fmt.Println(p.Reduce(ctx, rolling.Percentile[int](99.9)))
fmt.Println(p.Reduce(ctx, rolling.FastPercentile[int](99.9)))
```

The `Count`, `Avg`, `Min`, `Max`, and `Sum` each perform their expected
computation. The `Percentile` aggregators first take the target percentile and
return an aggregating function.

For cases of very large datasets, the `FastPercentile` can be used as a
replacement for the standard percentile calculation. This alternative version
uses the p-squared algorithm for estimating the percentile by processing
only one value at a time, in any order. The results are quite accurate but can
vary from the *actual* percentile by a small amount. It's a tradeoff of accuracy
for speed when calculating percentiles from large data sets. For more on the
p-squared algorithm see: <http://www.cs.wustl.edu/~jain/papers/ftp/psqr.pdf>.

#### Custom Aggregations

Any function that matches the form of
`func[T rolling.Numeric](context.Context, rolling.Window[T]) T` may be given to
the `Reduce` method of any window policy. The `Window[T]` type is a named
version of `[][]T` where T may be any integer or float type. Calling
`len(window)` will return the number of buckets. Each bucket is, itself, a slice
of `T` where `len(bucket)` is the number of values measured within that bucket.
Most aggregates will take the form of:

```golang
func MyAggregate[T rolling.Numeric](ctx context.Context, w rolling.Window[T]) T {
  for _, bucket := range w {
    for _, value := range bucket {
      // aggregate something
    }
  }
}
```

## Performance Considerations

Generally, the window policies leverage mostly fixed sized windows. For example,
point policies only contain up to one data point per bucket in the window which
means the memory usage is constant and there are no allocations for any number
of data points added to a point policy.

Time based policies, however, can record an unbounded number of data points per
bucket even if they maintain a fixed number of buckets in the window. To better
amortize the cost of allocating new space within buckets to contain more data,
time based policies will always re-use previous allocations rather than
returning them to garbage collection. A time policy in use for extended periods
of time tends to average down to zero allocations (as demonstrated in the
benchmarks included with the project). To further reduce runtime allocations you
can use the `NewPreallocatedWindow(buckets, bucketSize)` to generate a window
where each bucket contains `bucketSize` number of spaces. If you match this
value to your peak data ingestion rate per unit of time associated with each
bucket then you can achieve zero allocation time policy usage.

Most of the provided reduction methods (Sum, Avg, Min, etc.) run in `O(n)` time,
where `n` is the number of data points in the window, and result in zero
allocations. The exception to this is the `Percentile` reduction method. The
`Percentile` reduction attempts to calculate a perfectly accurate percentile
which requires it to sort all values in the window. The sort algorithm
complexity is documented in the
[sort.Stable](https://go.dev/pkg/sort/?m=old#Stable) documentation. The
`FastPercentile` reduction is `O(n)` and results in zero allocations but it uses
an estimation algorithm called [p-squared](http://www.cs.wustl.edu/~jain/papers/ftp/psqr.pdf)
that produces smaller errors when given larger datasets. See the p-squared paper
for more details.

Another performance factor to consider is that window policies use a mutex lock
to guard all reads and writes of the enclosed window. This means that reduction
methods hold the lock for the duration of their run. A more complex than usual
reduction or even a simple `O(n)` reduction over a very large dataset could mean
blocking. If reductions begin to cause a performance issue then the options
under the current design are limited to reducing the complexity of the
reduction, reducing the size of the dataset being reduced, and amortizing the
reduction cost by, for example, only running it every `T` amount of time or `C`
calls to reduce.

## Contributors

Pull requests, issues and comments welcome. For pull requests:

*   Add tests for new features and bug fixes
*   Follow the existing style
*   Separate unrelated changes into multiple pull requests

See the existing issues for things to start contributing.

For bigger changes, make sure you start a discussion first by creating
an issue and explaining the intended change.

## License

This project is licensed under the Apache 2.0 license. See
[LICENSE.txt](LICENSE.txt) or <http://www.apache.org/licenses/LICENSE-2.0> for
the full terms.

This project is forked from <https://github.com/asecurityteam/rolling>. The
original project's copyright attribution and license terms are:
```
Copyright @ 2017 Atlassian Pty Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

All files are marked with SPDX tags that both attribute the original copyright
as well as identify the author(s) of any significant changes to those files.
