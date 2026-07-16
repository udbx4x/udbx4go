package dataset

import (
	"context"
	stderrors "errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestEnvelopeCacheConcurrentBuildPublishesOnce(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, 1024, 2048)
	start := make(chan struct{})
	release := make(chan struct{})
	var builds atomic.Int32
	build := func(ctx context.Context, buffer *envelopeCacheBuildBuffer) error {
		builds.Add(1)
		close(start)
		select {
		case <-release:
			for _, entry := range []envelopeEntry{
				{ID: 3, MinX: 20, MinY: 20, MaxX: 30, MaxY: 30},
				{ID: 1, MinX: 0, MinY: 0, MaxX: 10, MaxY: 10},
				{ID: 2, MinX: 10, MinY: 5, MaxX: 12, MaxY: 8},
			} {
				if err := buffer.Append(entry); err != nil {
					return err
				}
			}
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	type result struct {
		cache *envelopeCache
		err   error
	}
	results := make(chan result, 2)
	go func() {
		cache, err := manager.GetOrBuild(context.Background(), "points", 3, build)
		results <- result{cache: cache, err: err}
	}()
	<-start
	go func() {
		cache, err := manager.GetOrBuild(context.Background(), "points", 3, build)
		results <- result{cache: cache, err: err}
	}()
	close(release)

	first := <-results
	second := <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	assert.Same(t, first.cache, second.cache)
	assert.Equal(t, int32(1), builds.Load())
	assert.True(t, first.cache.Complete())
	assert.Equal(t, int64(3)*envelopeEntryBytes, manager.TotalBytes())

	ids, hasMore, err := first.cache.CandidateIDs(types.BoundingBox{MinX: 10, MinY: 5, MaxX: 10, MaxY: 5}, 10)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, ids, "cache filtering must include boundary contacts in stable SmID order")
	assert.False(t, hasMore)
}

func TestEnvelopeCacheRejectsEstimatedBudgetWithoutScanning(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, envelopeEntryBytes, envelopeEntryBytes*10)
	called := false

	cache, err := manager.GetOrBuild(context.Background(), "too-large", 2, func(context.Context, *envelopeCacheBuildBuffer) error {
		called = true
		return nil
	})

	assert.Nil(t, cache)
	assert.ErrorIs(t, err, errEnvelopeCacheBudgetExceeded)
	assert.False(t, called)
	assert.Zero(t, manager.TotalBytes())
	assert.Zero(t, manager.ReservedBytes())
}

func TestEnvelopeCacheTotalBudgetDoesNotEvictPublishedCache(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, envelopeEntryBytes*2, envelopeEntryBytes*2)
	first, err := manager.GetOrBuild(context.Background(), "first", 2, fixedEnvelopeBuild(1, 2))
	require.NoError(t, err)

	called := false
	second, err := manager.GetOrBuild(context.Background(), "second", 1, func(_ context.Context, buffer *envelopeCacheBuildBuffer) error {
		called = true
		return buffer.Append(envelopeEntry{ID: 3})
	})

	assert.Nil(t, second)
	assert.ErrorIs(t, err, errEnvelopeCacheBudgetExceeded)
	assert.False(t, called)
	assert.Equal(t, envelopeEntryBytes*2, manager.TotalBytes())
	again, err := manager.GetOrBuild(context.Background(), "first", 2, fixedEnvelopeBuild(99))
	require.NoError(t, err)
	assert.Same(t, first, again)
}

func TestEnvelopeCacheGrowthRespectsTotalBudgetWithoutEviction(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, envelopeEntryBytes*4, envelopeEntryBytes*4)
	first, err := manager.GetOrBuild(context.Background(), "first", 1, fixedEnvelopeBuild(1))
	require.NoError(t, err)

	second, err := manager.GetOrBuild(context.Background(), "second", 1, fixedEnvelopeBuild(2, 3, 4, 5))

	assert.Nil(t, second)
	assert.ErrorIs(t, err, errEnvelopeCacheBudgetExceeded)
	assert.Equal(t, envelopeEntryBytes, manager.TotalBytes())
	assert.Zero(t, manager.ReservedBytes())
	assert.Equal(t, 1, manager.EntryCount())
	again, err := manager.GetOrBuild(context.Background(), "first", 1, fixedEnvelopeBuild(99))
	require.NoError(t, err)
	assert.Same(t, first, again)
}

func TestEnvelopeCacheFailureNeverPublishesOrLeaksReservation(t *testing.T) {
	tests := []struct {
		name  string
		build envelopeCacheBuildFunc
		check func(*testing.T, error)
	}{
		{
			name: "canceled",
			build: func(ctx context.Context, _ *envelopeCacheBuildBuffer) error {
				<-ctx.Done()
				return ctx.Err()
			},
			check: func(t *testing.T, err error) { assert.ErrorIs(t, err, context.Canceled) },
		},
		{
			name: "corrupt header",
			build: func(context.Context, *envelopeCacheBuildBuffer) error {
				return stderrors.New("corrupt header")
			},
			check: func(t *testing.T, err error) { assert.EqualError(t, err, "corrupt header") },
		},
		{
			name: "growth exceeds dataset budget",
			build: func(_ context.Context, buffer *envelopeCacheBuildBuffer) error {
				for id := int64(1); id <= 3; id++ {
					if err := buffer.Append(envelopeEntry{ID: id}); err != nil {
						return err
					}
				}
				return nil
			},
			check: func(t *testing.T, err error) { assert.ErrorIs(t, err, errEnvelopeCacheBudgetExceeded) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := newTestEnvelopeCacheManager(t, envelopeEntryBytes*2, envelopeEntryBytes*4)
			ctx := context.Background()
			if tt.name == "canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			cache, err := manager.GetOrBuild(ctx, "points", 1, tt.build)

			assert.Nil(t, cache)
			tt.check(t, err)
			assert.Zero(t, manager.TotalBytes())
			assert.Zero(t, manager.ReservedBytes())
			assert.Zero(t, manager.EntryCount())
		})
	}
}

func TestEnvelopeCacheBuildTimeoutReleasesReservation(t *testing.T) {
	manager, err := NewEnvelopeCacheManager(types.SpatialQueryPolicy{
		MaxDatasetCacheBytes: envelopeEntryBytes * 2,
		MaxTotalCacheBytes:   envelopeEntryBytes * 4,
		BuildTimeout:         10 * time.Millisecond,
	})
	require.NoError(t, err)

	cache, err := manager.GetOrBuild(context.Background(), "points", 1, func(ctx context.Context, _ *envelopeCacheBuildBuffer) error {
		<-ctx.Done()
		return ctx.Err()
	})

	assert.Nil(t, cache)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Zero(t, manager.TotalBytes())
	assert.Zero(t, manager.ReservedBytes())
}

func TestEnvelopeCacheCancellationDuringBuildReleasesReservation(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, envelopeEntryBytes*2, envelopeEntryBytes*4)
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	type buildResult struct {
		cache *envelopeCache
		err   error
	}
	result := make(chan buildResult, 1)
	go func() {
		cache, err := manager.GetOrBuild(ctx, "points", 1, func(buildCtx context.Context, _ *envelopeCacheBuildBuffer) error {
			close(started)
			<-buildCtx.Done()
			return buildCtx.Err()
		})
		result <- buildResult{cache: cache, err: err}
	}()
	<-started
	require.Equal(t, envelopeEntryBytes, manager.ReservedBytes())

	cancel()

	got := <-result
	assert.Nil(t, got.cache)
	assert.ErrorIs(t, got.err, context.Canceled)
	assert.Zero(t, manager.TotalBytes())
	assert.Zero(t, manager.ReservedBytes())
	assert.Zero(t, manager.EntryCount())
}

func TestEnvelopeCacheCancellationAfterBuildWakesWaiterWithoutPublishing(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, envelopeEntryBytes*4, envelopeEntryBytes*8)
	beforePublish := make(chan struct{})
	waiterJoined := make(chan struct{})
	var beforePublishOnce sync.Once
	var waiterOnce sync.Once
	manager.testHooks = &envelopeCacheManagerTestHooks{
		beforePublish: func(ctx context.Context) {
			beforePublishOnce.Do(func() { close(beforePublish) })
			<-ctx.Done()
		},
		waiterJoined: func() {
			waiterOnce.Do(func() { close(waiterJoined) })
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	type buildResult struct {
		cache *envelopeCache
		err   error
	}
	results := make(chan buildResult, 2)
	var builds atomic.Int32
	build := func(_ context.Context, buffer *envelopeCacheBuildBuffer) error {
		builds.Add(1)
		return buffer.Append(envelopeEntry{ID: 1})
	}
	go func() {
		cache, err := manager.GetOrBuild(ctx, "points", 1, build)
		results <- buildResult{cache: cache, err: err}
	}()
	<-beforePublish
	go func() {
		cache, err := manager.GetOrBuild(context.Background(), "points", 1, build)
		results <- buildResult{cache: cache, err: err}
	}()
	<-waiterJoined

	cancel()

	first := <-results
	second := <-results
	assert.Nil(t, first.cache)
	assert.Nil(t, second.cache)
	assert.ErrorIs(t, first.err, context.Canceled)
	assert.ErrorIs(t, second.err, context.Canceled)
	assert.Equal(t, int32(1), builds.Load())
	assert.Zero(t, manager.TotalBytes())
	assert.Zero(t, manager.ReservedBytes())
	assert.Zero(t, manager.EntryCount())
}

func TestEnvelopeCacheCloseClearsPublishedAndBuildingState(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, envelopeEntryBytes*4, envelopeEntryBytes*8)
	_, err := manager.GetOrBuild(context.Background(), "published", 1, fixedEnvelopeBuild(1))
	require.NoError(t, err)

	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, buildErr := manager.GetOrBuild(context.Background(), "building", 1, func(ctx context.Context, _ *envelopeCacheBuildBuffer) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
		finished <- buildErr
	}()
	<-started
	require.Positive(t, manager.ReservedBytes())

	manager.Close()

	assert.Error(t, <-finished)
	assert.Zero(t, manager.TotalBytes())
	assert.Zero(t, manager.ReservedBytes())
	assert.Zero(t, manager.EntryCount())
	cache, err := manager.GetOrBuild(context.Background(), "after-close", 1, fixedEnvelopeBuild(2))
	assert.Nil(t, cache)
	assert.ErrorIs(t, err, errEnvelopeCacheClosed)
}

func TestEnvelopeCacheRejectsObjectCountOverflow(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, math.MaxInt64, math.MaxInt64)
	called := false

	cache, err := manager.GetOrBuild(context.Background(), "overflow", math.MaxInt, func(context.Context, *envelopeCacheBuildBuffer) error {
		called = true
		return nil
	})

	assert.Nil(t, cache)
	assert.ErrorIs(t, err, errEnvelopeCacheBudgetExceeded)
	assert.False(t, called)
}

func newTestEnvelopeCacheManager(t *testing.T, datasetBytes, totalBytes int64) *EnvelopeCacheManager {
	t.Helper()
	manager, err := NewEnvelopeCacheManager(types.SpatialQueryPolicy{
		MaxDatasetCacheBytes: datasetBytes,
		MaxTotalCacheBytes:   totalBytes,
		BuildTimeout:         time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(manager.Close)
	return manager
}

func fixedEnvelopeBuild(ids ...int64) envelopeCacheBuildFunc {
	return func(_ context.Context, buffer *envelopeCacheBuildBuffer) error {
		for _, id := range ids {
			if err := buffer.Append(envelopeEntry{ID: id, MinX: float64(id), MinY: float64(id), MaxX: float64(id), MaxY: float64(id)}); err != nil {
				return err
			}
		}
		return nil
	}
}

func TestEnvelopeCacheCandidateIDsRejectsInvalidLimitAndID(t *testing.T) {
	cache := &envelopeCache{entries: []envelopeEntry{{ID: 0}}, complete: true}
	_, _, err := cache.CandidateIDs(types.BoundingBox{}, 0)
	assert.Error(t, err)

	_, _, err = cache.CandidateIDs(types.BoundingBox{}, 1)
	assert.Error(t, err)
}

func TestEnvelopeCacheConcurrentWaiterCanCancelWithoutCancelingBuilder(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, envelopeEntryBytes*2, envelopeEntryBytes*4)
	started := make(chan struct{})
	release := make(chan struct{})
	build := func(ctx context.Context, buffer *envelopeCacheBuildBuffer) error {
		close(started)
		select {
		case <-release:
			return buffer.Append(envelopeEntry{ID: 1})
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	built := make(chan *envelopeCache, 1)
	go func() {
		cache, _ := manager.GetOrBuild(context.Background(), "points", 1, build)
		built <- cache
	}()
	<-started

	waiterCtx, cancel := context.WithCancel(context.Background())
	cancel()
	cache, err := manager.GetOrBuild(waiterCtx, "points", 1, build)
	assert.Nil(t, cache)
	assert.ErrorIs(t, err, context.Canceled)

	close(release)
	assert.NotNil(t, <-built)
}

func TestEnvelopeCacheConcurrentWaitersReceiveSameBuildError(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, envelopeEntryBytes*2, envelopeEntryBytes*4)
	started := make(chan struct{})
	release := make(chan struct{})
	wantErr := stderrors.New("stable build error")
	var once sync.Once
	build := func(context.Context, *envelopeCacheBuildBuffer) error {
		once.Do(func() { close(started) })
		<-release
		return wantErr
	}

	errs := make(chan error, 2)
	go func() { _, err := manager.GetOrBuild(context.Background(), "points", 1, build); errs <- err }()
	<-started
	go func() { _, err := manager.GetOrBuild(context.Background(), "points", 1, build); errs <- err }()
	close(release)

	assert.ErrorIs(t, <-errs, wantErr)
	assert.ErrorIs(t, <-errs, wantErr)
}
