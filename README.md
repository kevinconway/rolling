# rolling

**A rolling window implementation for Go.**

[![Go Reference](https://pkg.go.dev/badge/github.com/kevinconway/rolling/v3.svg)](https://pkg.go.dev/github.com/kevinconway/rolling/v3)

- [rolling](#rolling)
  - [Installing](#installing)
  - [Rolling Time Windows](#rolling-time-windows)
    - [Time Window Concurrency Safety](#time-window-concurrency-safety)
    - [Time Window With Manual Timestamp Selection](#time-window-with-manual-timestamp-selection)
    - [Out Of Order Timestamps](#out-of-order-timestamps)
  - [Point Window](#point-window)
    - [Point Window Concurrency Safety](#point-window-concurrency-safety)
  - [Aggregating Windows](#aggregating-windows)
    - [Custom Aggregations](#custom-aggregations)
  - [Performance Considerations](#performance-considerations)
    - [Bucket Size And Allocations](#bucket-size-and-allocations)
    - [Reduction Method Complexity](#reduction-method-complexity)
    - [Cost Of Concurrency Safety](#cost-of-concurrency-safety)
  - [Fork Of github.com/asecurityteam/rolling](#fork-of-githubcomasecurityteamrolling)
    - [Drop-in Replacement Of github.com/asecurityteam/rolling](#drop-in-replacement-of-githubcomasecurityteamrolling)
    - [Migrating From github.com/asecurityteam/rolling](#migrating-from-githubcomasecurityteamrolling)
  - [Development](#development)
  - [Contributors](#contributors)
  - [License](#license)

## Installing

`go get github.com/kevinconway/rolling/v3`

## Rolling Time Windows

Rolling time windows represent a view of data from the current point in time to
some configurable duration in the past.

```golang
var p = rolling.NewTimePolicy[int](rolling.NewWindow[int](3000), time.Millisecond)
var start = time.Now()
for range time.Tick(time.Millisecond) {
  if time.Since(start) > 3*time.Second {
    break
  }
  p.Append(1)
}
```

The above creates a time window that contains 3,000 buckets where each bucket
contains, at most, 1ms of recorded data. New values are always recorded in the
most recent bucket and the oldest bucket is dropped in intervals of the bucket
duration. In the above, for example, the oldest bucket is dropped every
millisecond and a new, most recent bucket is created to collect values.

This type of window policy is most useful for collecting real-time values such
as request rates, error rates, and latency of operations.

### Time Window Concurrency Safety

The `NewTimePolicy` constructor returns a lock free implementation that is not
safe for concurrent use. Use `NewTimePolicyConcurrent` if you need to manage
concurrent readers and writers.

### Time Window With Manual Timestamp Selection

The time based rolling window defaults to using `time.Now()` for all timestamp
generation but it is possible to provide your own timestamps using the
`AppendWithTimestamp` and `ReduceWithTimestamp` variants:
```golang
var p = rolling.NewTimeWindow[int](rolling.NewWindow[int](3000), time.Millisecond)
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
ordered event stream where each event has an embedded timestamp. You should not
use both the standard and  `WithTimestamp` method variants for the same window.
The reason for this is that the time based policy is driven by the largest given
timestamp such that the general expectation is for timestamps to only increase
in value. Mixing custom and default timestamps can result in unexpected behavior
or a corrupt internal state of the policy.

### Out Of Order Timestamps

The risk of feeding out of order timestamps to a time based policy may be
greater when using custom timestamps but there is still a risk when using the
default timestamp generation. The current implementation leverages wall clock
values to determine passage of time and when routing values to window buckets.
There are multiple ways for wall clock time to run backwards depending on system
configuration and load on the system. Here's how the time policy handles
different ordering conditions:

- If the timestamp is older than the oldest bucket in the window then it is
  discarded.
- If the timestamp is within the current window then it is added to the
  appropriate bucket.
- If the timestamp is in the future compared to the current window then the
  future timestamp becomes the new leading edge of the window and any existing
  buckets that are now outside the window are discarded.

In effect, time windows can only move forward and never backwards. If time runs
backwards substantially then it is likely that all data points will be discarded
until the timestamp values return to the range within the window. If time runs
forward substantially then it will both discard all current values in the window
and set a new anchor point. The window never adjusts backwards so a future
timestamp effectively invalidates the window until the source of timestamps
begins producing values within the window established by the future timestamp.

While these timing conditions are possible, a substantial forward or backwards
move in wall clock time is uncommon and would likely have other negative effects
on a system beyond this library.

## Point Window

Rolling point windows represent a view of the most recently added data points.

```golang
var p = rolling.NewPointPolicy[int](rolling.NewWindow[int](5))

for x := 0; x < 5; x = x + 1 {
  p.Append(1)
}
p.Reduce(func(w Window[int]) int {
  fmt.Println(w) // [ [1] [1] [1] [1] [1] ]
  return 0
})
w.Append(5)
p.Reduce(func(w Window[int]) int {
  fmt.Println(w) // [ [5] [1] [1] [1] [1] ]
  return 0
})
w.Append(6)
p.Reduce(func(w Window[int]) int {
  fmt.Println(w) // [ [6] [5] [1] [1] [1] ]
  return 0
})
```

The above creates a window that always contains 5 data points. When the next
value is appended it will overwrite the first value. The window continuously
overwrites the oldest value with the newest to preserve the specified value
count. This type of window is similar to a circular buffer and is useful for
collecting data that have a known interval on which they are captured or for
tracking data where time is not a factor.

### Point Window Concurrency Safety

The `NewPointPolicy` constructor returns a lock free implementation that is not
safe for concurrent use. Use `NewPointPolicyConcurrent` if you need to manage
concurrent readers and writers.

## Aggregating Windows

Each window policy exposes a method with the signature
`Reduce(func(ctx context.Context, w Window[T]) T) T` that can be used to
aggregate the data stored within the window. The method takes in a function that
can compute the contents of the `Window` into a single value. For convenience,
this package provides some common reductions:

```golang
p.Reduce(ctx, rolling.Count[int])
p.Reduce(ctx, rolling.Avg[int])
p.Reduce(ctx, rolling.Min[int])
p.Reduce(ctx, rolling.Max[int])
p.Reduce(ctx, rolling.Sum[int])
p.Reduce(ctx, rolling.Percentile[int](99.9))
p.Reduce(ctx, rolling.FastPercentile[int](99.9))
```

The `Count`, `Avg`, `Min`, `Max`, and `Sum` each perform their expected
computation. The `Percentile` reduction can be constructed based on a target
percentile.

For cases of very large datasets, the `FastPercentile` can be used as a
replacement for the standard percentile calculation. This alternative version
uses the p-squared algorithm for estimating the percentile by processing
only one value at a time, in any order. The results are quite accurate but can
vary from the *actual* percentile by a small amount. It's a tradeoff of accuracy
for speed when calculating percentiles from large data sets. For more on the
p-squared algorithm see: <http://www.cs.wustl.edu/~jain/papers/ftp/psqr.pdf>.

### Custom Aggregations

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

### Bucket Size And Allocations

Generally, the window policies leverage fixed sized windows. For example, point
policies only contain up to one data point per bucket in the window which means
the memory usage is constant and there are no allocations for any number of data
points added to a point policy.

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

### Reduction Method Complexity

Most of the provided reduction methods (Sum, Avg, Min, etc.) run in `O(n)` time,
where `n` is the number of data points in the window, and result in zero
allocations. The `Count` reduction runs in `O(n)` but with `n` equal to the
number of buckets. The `Percentile` reduction, however, attempts to calculate a
perfectly accurate percentile which requires it to sort all values in the
window. The sort algorithm complexity is documented in the
[sort.Stable](https://go.dev/pkg/sort/?m=old#Stable) documentation. The
`FastPercentile` reduction is `O(n)` and results in zero allocations but it uses
an estimation algorithm called [p-squared](http://www.cs.wustl.edu/~jain/papers/ftp/psqr.pdf)
that produces smaller errors when given larger datasets. See the p-squared paper
for more details.

### Cost Of Concurrency Safety

Another performance factor to consider is that concurrency safe window policies
use a mutex lock to guard all reads and writes of the enclosed window. This
means that reduction methods hold the lock for the duration of their run. A more
complex than usual reduction or even a simple `O(n)` reduction over a very large
dataset could mean blocking. If reductions begin to cause a performance issue
then the options under the current design are limited to reducing the complexity
of the reduction, reducing the size of the dataset being operated on, and
amortizing the reduction cost by, for example, only running it in intervals of
time or after a fixed amount of calls that get a cached result. 

## Fork Of github.com/asecurityteam/rolling

In the past, I was part of the original team that built and maintained
github.com/asecurityteam/rolling which was used in service reliability and
performance tooling. Since then, the team's priorities and tooling have changed.

I have new use cases for this library so I'm maintaining this fork. This version
includes support for multiple numeric types, a small number of bug fixes, and
a few small performance optimizations.

### Drop-in Replacement Of github.com/asecurityteam/rolling

For convenience, I have created a `v2.2.1` tag that matches the last published
release of github.com/asecurityteam/rolling. The only difference is that I
have updated the module path to `github.com/kevinconway/rolling/v2`. You should
be able to replace `github.com/asecurityteam/rolling` with
`github.com/kevinconway/rolling/v2` in either your source code or your go.mod
using a `replace` directive to pull from here instead.

There is no particular advantage to doing this, today, unless it's part of a
gradual migration to v3 (the version documented in this file). If someone finds
a severe enough bug then I may release an additional v2 patch.

### Migrating From github.com/asecurityteam/rolling

This fork of the project has a nearly identical interface to the original except
that this version uses Go generics to allow all numeric types in a window rather
than only `float64`. To update, you should only need to add a type parameter to
constructor methods and reductions. For example:
```go
p := rolling.NewTimeWindow(rolling.NewWindow(3000), time.Millisecond)
p.Reduce(rolling.Sum)
```
becomes:
```go
p := rolling.NewTimeWindow[int](rolling.NewWindow[int](3000), time.Millisecond)
p.Reduce(ctx, rolling.Sum[int])
```

## Development

This project has no hard dependencies on any build tools other than Go. You
should be able to run `go test` for any changes and see the results.

If you prefer, the project includes a Makefile with the following rules:

- `update` - Update dependencies in the go.mod file.
- `bin` - Download all optional build and test tools.
- `fmt` - Run `goimports` on all Go source files.
- `test` - Run all tests and create a test coverage record.
- `coverage` - Generate a series of coverage reports from test records.

## Contributors

For bugs or performance improvements, I welcome pull requests, issues, or
comments. If you make a pull request then please be sure to add tests and run
`make fmt`.

For new features, please start a discussion first by creating an issue and
explaining the intended change.

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
