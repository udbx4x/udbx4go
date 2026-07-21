package udbx4go

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/udbx4x/udbx4go/internal/dataset"
	"github.com/udbx4x/udbx4go/pkg/types"
)

var realHenanSpatialBenchmarkBounds = BoundingBox{
	MinX: 113.5,
	MinY: 34.5,
	MaxX: 114.0,
	MaxY: 35.0,
}

func BenchmarkRealHenanWeiboSpatialQuery(b *testing.B) {
	ds := openRealHenanReadOnly(b)
	defer ds.Close()
	options := SpatialQueryOptions{Bounds: realHenanSpatialBenchmarkBounds, Limit: 1000}

	warm, err := ds.QuerySpatial(context.Background(), "weibo", options)
	if err != nil {
		b.Fatalf("warm weibo spatial query: %v", err)
	}
	if warm.Strategy != SpatialQueryStrategyRTree {
		b.Fatalf("unexpected weibo strategy: %s", warm.Strategy)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := ds.QuerySpatial(context.Background(), "weibo", options)
		if err != nil {
			b.Fatalf("query weibo viewport: %v", err)
		}
		if result.Strategy != SpatialQueryStrategyRTree {
			b.Fatalf("unexpected weibo strategy: %s", result.Strategy)
		}
	}
}

func BenchmarkRealHenanCountyEnvelopeQuery(b *testing.B) {
	options := SpatialQueryOptions{Bounds: realHenanSpatialBenchmarkBounds, Limit: 100}

	b.Run("cold_build", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			ds := openRealHenanReadOnly(b)
			b.StartTimer()
			result, err := ds.QuerySpatial(context.Background(), "县级行政区划", options)
			b.StopTimer()
			if err != nil {
				_ = ds.Close()
				b.Fatalf("build county envelope cache: %v", err)
			}
			if result.Strategy != SpatialQueryStrategyEnvelopeCache {
				_ = ds.Close()
				b.Fatalf("unexpected county strategy: %s", result.Strategy)
			}
			if err := ds.Close(); err != nil {
				b.Fatalf("close county data source: %v", err)
			}
		}
	})

	b.Run("hot_filter", func(b *testing.B) {
		ds := openRealHenanReadOnly(b)
		defer ds.Close()
		warm, err := ds.QuerySpatial(context.Background(), "县级行政区划", options)
		if err != nil {
			b.Fatalf("warm county envelope cache: %v", err)
		}
		if warm.Strategy != SpatialQueryStrategyEnvelopeCache {
			b.Fatalf("unexpected county strategy: %s", warm.Strategy)
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result, err := ds.QuerySpatial(context.Background(), "县级行政区划", options)
			if err != nil {
				b.Fatalf("filter county envelope cache: %v", err)
			}
			if result.Strategy != SpatialQueryStrategyEnvelopeCache {
				b.Fatalf("unexpected county strategy: %s", result.Strategy)
			}
		}
	})
}

func BenchmarkRealHenanOpen(b *testing.B) {
	path := filepath.Join("..", "data", "henan.udbx")

	for i := 0; i < b.N; i++ {
		ds, err := Open(path)
		if err != nil {
			b.Fatalf("open henan.udbx: %v", err)
		}
		if err := ds.Close(); err != nil {
			b.Fatalf("close henan.udbx: %v", err)
		}
	}
}

func BenchmarkRealHenanWeiboCount(b *testing.B) {
	ds, dataset := openBenchmarkWeiboDataset(b)
	defer ds.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := dataset.Count()
		if err != nil {
			b.Fatalf("count weibo dataset: %v", err)
		}
		if got != 469308 {
			b.Fatalf("unexpected count: %d", got)
		}
	}
}

func BenchmarkRealHenanGetWeiboDataset(b *testing.B) {
	path := filepath.Join("..", "data", "henan.udbx")
	ds, err := Open(path)
	if err != nil {
		b.Fatalf("open henan.udbx: %v", err)
	}
	defer ds.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dataset, err := ds.GetPointDataset("weibo")
		if err != nil {
			b.Fatalf("get weibo dataset: %v", err)
		}
		if dataset.Info().TableName != "henan_P" {
			b.Fatalf("unexpected table name: %s", dataset.Info().TableName)
		}
	}
}

func BenchmarkRealHenanWeiboFirstPage3(b *testing.B) {
	ds, dataset := openBenchmarkWeiboDataset(b)
	defer ds.Close()

	options := &types.QueryOptions{Limit: 3, Offset: 0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		features, err := dataset.List(options)
		if err != nil {
			b.Fatalf("list first page: %v", err)
		}
		if len(features) != 3 {
			b.Fatalf("unexpected feature count: %d", len(features))
		}
	}
}

func BenchmarkRealHenanWeiboSecondPage3(b *testing.B) {
	ds, dataset := openBenchmarkWeiboDataset(b)
	defer ds.Close()

	options := &types.QueryOptions{Limit: 3, Offset: 3}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		features, err := dataset.List(options)
		if err != nil {
			b.Fatalf("list second page: %v", err)
		}
		if len(features) != 3 {
			b.Fatalf("unexpected feature count: %d", len(features))
		}
	}
}

func BenchmarkRealHenanWeiboFirstPage100(b *testing.B) {
	ds, dataset := openBenchmarkWeiboDataset(b)
	defer ds.Close()

	options := &types.QueryOptions{Limit: 100, Offset: 0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		features, err := dataset.List(options)
		if err != nil {
			b.Fatalf("list first page 100: %v", err)
		}
		if len(features) != 100 {
			b.Fatalf("unexpected feature count: %d", len(features))
		}
	}
}

func BenchmarkRealHenanWeiboDeepPage100(b *testing.B) {
	ds, dataset := openBenchmarkWeiboDataset(b)
	defer ds.Close()

	options := &types.QueryOptions{Limit: 100, Offset: 100000}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		features, err := dataset.List(options)
		if err != nil {
			b.Fatalf("list deep page: %v", err)
		}
		if len(features) != 100 {
			b.Fatalf("unexpected feature count: %d", len(features))
		}
	}
}

func BenchmarkRealHenanWeiboTenPages100(b *testing.B) {
	ds, dataset := openBenchmarkWeiboDataset(b)
	defer ds.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total := 0
		for page := 0; page < 10; page++ {
			features, err := dataset.List(&types.QueryOptions{Limit: 100, Offset: page * 100})
			if err != nil {
				b.Fatalf("list page %d: %v", page, err)
			}
			total += len(features)
		}
		if total != 1000 {
			b.Fatalf("unexpected total feature count: %d", total)
		}
	}
}

func openBenchmarkWeiboDataset(b *testing.B) (*DataSource, *dataset.PointDataset) {
	b.Helper()

	ds, err := Open(filepath.Join("..", "data", "henan.udbx"))
	if err != nil {
		b.Fatalf("open henan.udbx: %v", err)
	}

	dataset, err := ds.GetPointDataset("weibo")
	if err != nil {
		_ = ds.Close()
		b.Fatalf("get weibo dataset: %v", err)
	}

	return ds, dataset
}
