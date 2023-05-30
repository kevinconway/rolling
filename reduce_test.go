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
	"testing"
)

// https://gist.github.com/cevaris/bc331cbe970b03816c6b
var epsilon = 0.00000001

func floatEquals(a float64, b float64) bool {
	return (a-b) < epsilon && (b-a) < epsilon
}

var largeEpsilon = 0.001

func floatMostlyEquals(a float64, b float64) bool {
	return (a-b) < largeEpsilon && (b-a) < largeEpsilon

}

func TestCount(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var numberOfPoints = 100
	var w = NewWindow[int](numberOfPoints)
	var p = NewPointPolicy(w)
	for x := 1; x <= numberOfPoints; x = x + 1 {
		p.Append(ctx, x)
	}
	var result = p.Reduce(ctx, Count[int])

	var expected = 100
	if result != expected {
		t.Fatalf("count calculated incorrectly: %d versus %d", expected, result)
	}
}

func TestCountPreallocatedWindow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var numberOfPoints = 100
	var w = NewPreallocatedWindow[int](numberOfPoints, 100)
	var p = NewPointPolicy(w)
	for x := 1; x <= numberOfPoints; x = x + 1 {
		p.Append(ctx, x)
	}
	var result = p.Reduce(ctx, Count[int])

	var expected = 100
	if result != expected {
		t.Fatalf("count with prealloc window calculated incorrectly: %d versus %d", expected, result)
	}
}

func TestSum(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var numberOfPoints = 100
	var w = NewWindow[int](numberOfPoints)
	var p = NewPointPolicy(w)
	for x := 1; x <= numberOfPoints; x = x + 1 {
		p.Append(ctx, x)
	}
	var result = p.Reduce(ctx, Sum[int])

	var expected = 5050
	if result != expected {
		t.Fatalf("avg calculated incorrectly: %d versus %d", expected, result)
	}
}

func TestSumFloat(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var numberOfPoints = 100
	var w = NewWindow[float64](numberOfPoints)
	var p = NewPointPolicy(w)
	for x := 1; x <= numberOfPoints; x = x + 1 {
		p.Append(ctx, float64(x))
	}
	var result = p.Reduce(ctx, Sum[float64])

	var expected = 5050.0
	if !floatEquals(result, expected) {
		t.Fatalf("avg calculated incorrectly: %f versus %f", expected, result)
	}
}

func TestAvg(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var numberOfPoints = 100
	var w = NewWindow[int](numberOfPoints)
	var p = NewPointPolicy(w)
	for x := 1; x <= numberOfPoints; x = x + 1 {
		p.Append(ctx, x)
	}
	var result = p.Reduce(ctx, Avg[int])

	var expected = 50
	if result != expected {
		t.Fatalf("avg calculated incorrectly: %d versus %d", expected, result)
	}
}

func TestAvgFloat(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var numberOfPoints = 100
	var w = NewWindow[float64](numberOfPoints)
	var p = NewPointPolicy(w)
	for x := 1; x <= numberOfPoints; x = x + 1 {
		p.Append(ctx, float64(x))
	}
	var result = p.Reduce(ctx, Avg[float64])

	var expected = 50.5
	if !floatEquals(result, expected) {
		t.Fatalf("avg calculated incorrectly: %f versus %f", expected, result)
	}
}

func TestMax(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var numberOfPoints = 100
	var w = NewWindow[int](numberOfPoints)
	var p = NewPointPolicy(w)
	for x := 1; x <= numberOfPoints; x = x + 1 {
		p.Append(ctx, 100.0-x)
	}
	var result = p.Reduce(ctx, Max[int])

	var expected = 99
	if result != expected {
		t.Fatalf("max calculated incorrectly: %d versus %d", expected, result)
	}
}

func TestMin(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var numberOfPoints = 100
	var w = NewWindow[int](numberOfPoints)
	var p = NewPointPolicy(w)
	for x := 1; x <= numberOfPoints; x = x + 1 {
		p.Append(ctx, x)
	}
	var result = p.Reduce(ctx, Min[int])

	var expected = 1
	if result != expected {
		t.Fatalf("Min calculated incorrectly: %d versus %d", expected, result)
	}
}

func TestPercentileAggregateInterpolateWhenEmpty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var numberOfPoints = 0
	var w = NewWindow[float64](numberOfPoints)
	var p = NewPointPolicy(w)
	var perc = 99.9
	var a = Percentile[float64](perc)
	var result = p.Reduce(ctx, a)
	if !floatEquals(result, 0) {
		t.Fatalf("percentile should be zero but got %f", result)
	}
}

func TestPercentileAggregateInterpolateWhenInsufficientData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var numberOfPoints = 100
	var w = NewWindow[float64](numberOfPoints)
	var p = NewPointPolicy(w)
	for x := 1; x <= numberOfPoints; x = x + 1 {
		p.Append(ctx, float64(x))
	}
	var perc = 99.9
	var a = Percentile[float64](perc)
	var result = p.Reduce(ctx, a)

	// When there are insufficient values to satisfy the precision then the
	// percentile algorithm degenerates to a max function. In this case, we need
	// 1000 values in order to select a 99.9 but only have 100. 100 is also the
	// maximum value and will be selected as k and k+1 in the linear
	// interpolation.
	var expected = 100.0
	if !floatEquals(result, expected) {
		t.Fatalf("%f percentile calculated incorrectly: %f versus %f", perc, expected, result)
	}
}

func TestPercentileAggregateInterpolateWhenSufficientData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var numberOfPoints = 1000
	var w = NewWindow[float64](numberOfPoints)
	var p = NewPointPolicy(w)
	for x := 1; x <= numberOfPoints; x = x + 1 {
		p.Append(ctx, float64(x))
	}
	var perc = 99.9
	var a = Percentile[float64](perc)
	var result = p.Reduce(ctx, a)
	var expected = 999.5
	if !floatEquals(result, expected) {
		t.Fatalf("%f percentile calculated incorrectly: %f versus %f", perc, expected, result)
	}
}

func TestFastPercentileAggregateInterpolateWhenEmpty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var numberOfPoints = 0
	var w = NewWindow[float64](numberOfPoints)
	var p = NewPointPolicy(w)
	var perc = 99.9
	var a = FastPercentile[float64](perc)
	var result = p.Reduce(ctx, a)
	if !floatEquals(result, 0) {
		t.Fatalf("fast percentile should be zero but got %f", result)
	}
}

func TestFastPercentileAggregateInterpolateWhenSufficientData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// Using a larger dataset so that the algorithm can converge on the
	// correct value. Smaller datasets where the value might be interpolated
	// linearly in the typical percentile calculation results in larger error
	// in the result. This is acceptable so long as the estimated value approaches
	// the correct value as more data are given.
	var numberOfPoints = 10000
	var w = NewWindow[float64](numberOfPoints)
	var p = NewPointPolicy(w)
	for x := 1; x <= numberOfPoints; x = x + 1 {
		p.Append(ctx, float64(x))
	}
	var perc = 99.9
	var a = FastPercentile[float64](perc)
	var result = p.Reduce(ctx, a)
	var expected = 9990.0
	if !floatEquals(result, expected) {
		t.Fatalf("%f percentile calculated incorrectly: %f versus %f", perc, expected, result)
	}
}

func TestFastPercentileAggregateUsingPSquaredDataSet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var numberOfPoints = 20
	var w = NewWindow[float64](numberOfPoints)
	var p = NewPointPolicy(w)
	p.Append(ctx, 0.02)
	p.Append(ctx, 0.15)
	p.Append(ctx, 0.74)
	p.Append(ctx, 0.83)
	p.Append(ctx, 3.39)
	p.Append(ctx, 22.37)
	p.Append(ctx, 10.15)
	p.Append(ctx, 15.43)
	p.Append(ctx, 38.62)
	p.Append(ctx, 15.92)
	p.Append(ctx, 34.60)
	p.Append(ctx, 10.28)
	p.Append(ctx, 1.47)
	p.Append(ctx, 0.40)
	p.Append(ctx, 0.05)
	p.Append(ctx, 11.39)
	p.Append(ctx, 0.27)
	p.Append(ctx, 0.42)
	p.Append(ctx, 0.09)
	p.Append(ctx, 11.37)
	var perc = 50.0
	var a = FastPercentile[float64](perc)
	var result = p.Reduce(ctx, a)
	var expected = 4.44
	if !floatMostlyEquals(result, expected) {
		t.Fatalf("%f percentile calculated incorrectly: %f versus %f", perc, expected, result)
	}
}

var aggregateResult float64

type policy interface {
	Append(context.Context, float64)
	Reduce(context.Context, Reduction[float64]) float64
}
type aggregateBench struct {
	inserts       int
	policy        policy
	aggregate     Reduction[float64]
	aggregateName string
}

func BenchmarkAggregates(b *testing.B) {
	var baseCases = []*aggregateBench{
		{aggregate: Sum[float64], aggregateName: "sum"},
		{aggregate: Min[float64], aggregateName: "min"},
		{aggregate: Max[float64], aggregateName: "max"},
		{aggregate: Avg[float64], aggregateName: "avg"},
		{aggregate: Count[float64], aggregateName: "count"},
		{aggregate: Percentile[float64](50.0), aggregateName: "p50"},
		{aggregate: Percentile[float64](99.9), aggregateName: "p99.9"},
		{aggregate: FastPercentile[float64](50.0), aggregateName: "fp50"},
		{aggregate: FastPercentile[float64](99.9), aggregateName: "fp99.9"},
	}
	var insertions = []int{1, 1000, 10000, 100000}
	var benchCases = make([]*aggregateBench, 0, len(baseCases)*len(insertions))
	ctx := context.Background()
	for _, baseCase := range baseCases {
		for _, inserts := range insertions {
			var w = NewWindow[float64](inserts)
			var p = NewPointPolicy(w)
			for x := 1; x <= inserts; x = x + 1 {
				p.Append(ctx, float64(x))
			}
			benchCases = append(benchCases, &aggregateBench{
				inserts:       inserts,
				aggregate:     baseCase.aggregate,
				aggregateName: baseCase.aggregateName,
				policy:        p,
			})
		}
	}

	for _, benchCase := range benchCases {
		b.Run(fmt.Sprintf("Aggregate:%s-DataPoints:%d", benchCase.aggregateName, benchCase.inserts), func(bt *testing.B) {
			var result float64
			bt.ResetTimer()
			for n := 0; n < bt.N; n = n + 1 {
				result = benchCase.policy.Reduce(ctx, benchCase.aggregate)
			}
			aggregateResult = result
		})
	}
}
