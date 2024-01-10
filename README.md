# rolling

**A rolling/sliding window implementation for Go.**

- [rolling](#rolling)
  - [Usage](#usage)
    - [Point Window](#point-window)
    - [Time Window](#time-window)
  - [Aggregating Windows](#aggregating-windows)
      - [Custom Aggregations](#custom-aggregations)
  - [Contributors](#contributors)
  - [License](#license)

## Usage

### Point Window

```golang
var p = rolling.NewPointPolicy(rolling.NewWindow(5))

for x := 0; x < 5; x = x + 1 {
  p.Append(x)
}
p.Reduce(func(w Window) float64 {
  fmt.Println(w) // [ [0] [1] [2] [3] [4] ]
  return 0
})
w.Append(5)
p.Reduce(func(w Window) float64 {
  fmt.Println(w) // [ [5] [1] [2] [3] [4] ]
  return 0
})
w.Append(6)
p.Reduce(func(w Window) float64 {
  fmt.Println(w) // [ [5] [6] [2] [3] [4] ]
  return 0
})
```

The above creates a window that always contains 5 data points and then fills
it with the values 0 - 4. When the next value is appended it will overwrite
the first value. The window continuously overwrites the oldest value with the
latest to preserve the specified value count. This type of window is useful
for collecting data that have a known interval on which they are capture or
for tracking data where time is not a factor.

### Time Window

```golang
var p = rolling.NewTimeWindow(rolling.NewWindow(3000), time.Millisecond)
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

## Aggregating Windows

Each window exposes a `Reduce(func(w Window) float64) float64` method that can
be used to aggregate the data stored within. The method takes in a function
that can compute the contents of the `Window` into a single value. For
convenience, this package provides some common reductions:

```golang
fmt.Println(p.Reduce(rolling.Count))
fmt.Println(p.Reduce(rolling.Avg))
fmt.Println(p.Reduce(rolling.Min))
fmt.Println(p.Reduce(rolling.Max))
fmt.Println(p.Reduce(rolling.Sum))
fmt.Println(p.Reduce(rolling.Percentile(99.9)))
fmt.Println(p.Reduce(rolling.FastPercentile(99.9)))
```

The `Count`, `Avg`, `Min`, `Max`, and `Sum` each perform their expected
computation. The `Percentile` aggregator first takes the target percentile and
returns an aggregating function that works identically to the `Sum`, et al.

For cases of very large datasets, the `FastPercentile` can be used as a
replacement for the standard percentile calculation. This alternative version
uses the p-squared algorithm for estimating the percentile by processing
only one value at a time, in any order. The results are quite accurate but can
vary from the *actual* percentile by a small amount. It's a tradeoff of accuracy
for speed when calculating percentiles from large data sets. For more on the
p-squared algorithm see: <http://www.cs.wustl.edu/~jain/papers/ftp/psqr.pdf>.

#### Custom Aggregations

Any function that matches the form of `func(rolling.Window)float64` may be given
to the `Reduce` method of any window policy. The `Window` type is a named
version of `[][]float64`. Calling `len(window)` will return the number of
buckets. Each bucket is, itself, a slice of floats where `len(bucket)` is the
number of values measured within that bucket. Most aggregate will take the form
of:

```golang
func MyAggregate(w rolling.Window) float64 {
  for _, bucket := range w {
    for _, value := range bucket {
      // aggregate something
    }
  }
}
```

## Contributors

Pull requests, issues and comments welcome. For pull requests:

*   Add tests for new features and bug fixes
*   Follow the existing style
*   Separate unrelated changes into multiple pull requests

See the existing issues for things to start contributing.

For bigger changes, make sure you start a discussion first by creating
an issue and explaining the intended change.

## License

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

The original files are marked with this attribution. All modifications are also
distributed under the Apache 2.0 license.
