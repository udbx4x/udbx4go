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
	udbxerrors "github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestEnvelopeCacheEstimatedRSSChargeMatchesPOCModel(t *testing.T) {
	assert.Equal(t, int64(4*1024*1024), envelopeCacheFixedRSSChargeBytes)
	assert.Equal(t, int64(80), envelopeCacheRSSChargePerCapacityEntry)

	measuredP95MiB := map[int]float64{
		10_000:  3.859,
		50_000:  7.703,
		100_000: 11.470,
		250_000: 23.190,
		500_000: 42.340,
	}
	for capacity, measured := range measuredP95MiB {
		charge, err := envelopeCacheRSSChargeForCapacity(capacity)
		require.NoError(t, err)
		expected := envelopeCacheFixedRSSChargeBytes + int64(capacity)*envelopeCacheRSSChargePerCapacityEntry
		assert.Equal(t, expected, charge)
		assert.InDelta(t, measured, float64(charge)/(1024*1024), 1.0, "capacity=%d", capacity)
	}
}

func TestEnvelopeCacheDefaultRSSGateAllows250KAndRejects500KBeforeBuild(t *testing.T) {
	policy := types.DefaultSpatialQueryPolicy()
	buildReached := stderrors.New("build reached")

	allowed, err := NewEnvelopeCacheManager(policy)
	require.NoError(t, err)
	t.Cleanup(allowed.Close)
	called := false
	cache, err := allowed.GetOrBuild(context.Background(), "250k", 250_000, func(context.Context, *envelopeCacheBuildBuffer) error {
		called = true
		return buildReached
	})
	assert.Nil(t, cache)
	assert.ErrorIs(t, err, buildReached)
	assert.True(t, called)

	rejected, err := NewEnvelopeCacheManager(policy)
	require.NoError(t, err)
	t.Cleanup(rejected.Close)
	called = false
	cache, err = rejected.GetOrBuild(context.Background(), "500k", 500_000, func(context.Context, *envelopeCacheBuildBuffer) error {
		called = true
		return nil
	})
	assert.Nil(t, cache)
	assert.ErrorIs(t, err, errEnvelopeCacheBudgetExceeded)
	assert.False(t, called)
}

func TestEnvelopeCacheConcurrentBuildPublishesOnce(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 3), testEnvelopeCacheRSSCharge(t, 3))
	start := make(chan struct{})
	release := make(chan struct{})
	waiterJoined := make(chan struct{})
	var waiterOnce sync.Once
	manager.testHooks = &envelopeCacheManagerTestHooks{
		waiterJoined: func() {
			waiterOnce.Do(func() { close(waiterJoined) })
		},
	}
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
	<-waiterJoined
	close(release)

	first := <-results
	second := <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	assert.Same(t, first.cache, second.cache)
	assert.Equal(t, int32(1), builds.Load())
	assert.True(t, first.cache.Complete())
	assert.Equal(t, testEnvelopeCacheRSSCharge(t, 3), manager.TotalBytes())

	ids, hasMore, err := first.cache.CandidateIDs(types.BoundingBox{MinX: 10, MinY: 5, MaxX: 10, MaxY: 5}, 10)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, ids, "cache filtering must include boundary contacts in stable SmID order")
	assert.False(t, hasMore)
}

func TestEnvelopeCacheRejectsEstimatedBudgetWithoutScanning(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 1), testEnvelopeCacheRSSCharge(t, 10))
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
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 2), testEnvelopeCacheRSSCharge(t, 2))
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
	assert.Equal(t, testEnvelopeCacheRSSCharge(t, 2), manager.TotalBytes())
	again, err := manager.GetOrBuild(context.Background(), "first", 2, fixedEnvelopeBuild(99))
	require.NoError(t, err)
	assert.Same(t, first, again)
}

func TestEnvelopeCacheGrowthRespectsTotalBudgetWithoutEviction(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 8), testEnvelopeCacheTotalRSSCharge(t, 1, 1))
	first, err := manager.GetOrBuild(context.Background(), "first", 1, fixedEnvelopeBuild(1))
	require.NoError(t, err)

	second, err := manager.GetOrBuild(context.Background(), "second", 1, fixedEnvelopeBuild(2, 3, 4, 5))

	assert.Nil(t, second)
	assert.ErrorIs(t, err, errEnvelopeCacheBudgetExceeded)
	assert.Equal(t, testEnvelopeCacheRSSCharge(t, 1), manager.TotalBytes())
	assert.Zero(t, manager.ReservedBytes())
	assert.Equal(t, 1, manager.EntryCount())
	again, err := manager.GetOrBuild(context.Background(), "first", 1, fixedEnvelopeBuild(99))
	require.NoError(t, err)
	assert.Same(t, first, again)
}

func TestEnvelopeCacheGrowthRejectsDatasetBackingPeakBeforeAllocation(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 2), testEnvelopeCacheRSSCharge(t, 10))
	buildCalls := 0
	capacityAfterRejection := -1

	cache, err := manager.GetOrBuild(context.Background(), "points", 1, func(
		_ context.Context,
		buffer *envelopeCacheBuildBuffer,
	) error {
		buildCalls++
		require.NoError(t, buffer.Append(envelopeEntry{ID: 1}))
		appendErr := buffer.Append(envelopeEntry{ID: 2})
		capacityAfterRejection = cap(buffer.entries)
		return appendErr
	})

	assert.Nil(t, cache)
	assert.ErrorIs(t, err, errEnvelopeCacheBudgetExceeded)
	assert.Equal(t, 1, buildCalls)
	assert.Equal(t, 1, capacityAfterRejection, "growth must be rejected before allocating the new backing array")
	assert.Zero(t, manager.TotalBytes())
	assert.Zero(t, manager.ReservedBytes())
	assert.Zero(t, manager.EntryCount())
}

func TestEnvelopeCacheGrowthRejectsTotalBackingPeakBeforeAllocation(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 3), testEnvelopeCacheTotalRSSCharge(t, 1, 1))
	blockerCtx, cancelBlocker := context.WithCancel(context.Background())
	blockerStarted := make(chan struct{})
	blockerFinished := make(chan error, 1)
	go func() {
		_, err := manager.GetOrBuild(blockerCtx, "blocker", 1, func(ctx context.Context, _ *envelopeCacheBuildBuffer) error {
			close(blockerStarted)
			<-ctx.Done()
			return ctx.Err()
		})
		blockerFinished <- err
	}()
	<-blockerStarted

	buildCalls := 0
	capacityAfterRejection := -1
	cache, err := manager.GetOrBuild(context.Background(), "points", 1, func(
		_ context.Context,
		buffer *envelopeCacheBuildBuffer,
	) error {
		buildCalls++
		require.NoError(t, buffer.Append(envelopeEntry{ID: 1}))
		appendErr := buffer.Append(envelopeEntry{ID: 2})
		capacityAfterRejection = cap(buffer.entries)
		return appendErr
	})

	assert.Nil(t, cache)
	assert.ErrorIs(t, err, errEnvelopeCacheBudgetExceeded)
	assert.Equal(t, 1, buildCalls)
	assert.Equal(t, 1, capacityAfterRejection, "growth must be rejected before allocating the new backing array")
	assert.Zero(t, manager.TotalBytes())
	assert.Equal(t, testEnvelopeCacheRSSCharge(t, 1), manager.ReservedBytes())
	assert.Zero(t, manager.EntryCount())

	cancelBlocker()
	assert.ErrorIs(t, <-blockerFinished, context.Canceled)
	assert.Zero(t, manager.TotalBytes())
	assert.Zero(t, manager.ReservedBytes())
}

func TestEnvelopeCacheGrowthReleasesPreviousBackingReservationAfterCopy(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 3), testEnvelopeCacheRSSCharge(t, 3))
	reservedAfterGrowth := int64(-1)

	cache, err := manager.GetOrBuild(context.Background(), "points", 1, func(
		_ context.Context,
		buffer *envelopeCacheBuildBuffer,
	) error {
		require.NoError(t, buffer.Append(envelopeEntry{ID: 1}))
		require.NoError(t, buffer.Append(envelopeEntry{ID: 2}))
		reservedAfterGrowth = manager.ReservedBytes()
		return udbxerrors.FormatError("stop after observing growth reservation")
	})

	assert.Nil(t, cache)
	assert.True(t, udbxerrors.IsFormatError(err))
	assert.Equal(t, testEnvelopeCacheRSSCharge(t, 2), reservedAfterGrowth)
	assert.Zero(t, manager.TotalBytes())
	assert.Zero(t, manager.ReservedBytes())
}

func TestEnvelopeCacheBackingPeakByteOverflowIsRejected(t *testing.T) {
	bytes, err := addEnvelopeCacheBytes(math.MaxInt64, 1)

	assert.Zero(t, bytes)
	assert.ErrorIs(t, err, errEnvelopeCacheBudgetExceeded)
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
			manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 2), testEnvelopeCacheRSSCharge(t, 4))
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
		MaxDatasetCacheBytes: testEnvelopeCacheRSSCharge(t, 2),
		MaxTotalCacheBytes:   testEnvelopeCacheRSSCharge(t, 4),
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
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 2), testEnvelopeCacheRSSCharge(t, 4))
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
	require.Equal(t, testEnvelopeCacheRSSCharge(t, 1), manager.ReservedBytes())

	cancel()

	got := <-result
	assert.Nil(t, got.cache)
	assert.ErrorIs(t, got.err, context.Canceled)
	assert.Zero(t, manager.TotalBytes())
	assert.Zero(t, manager.ReservedBytes())
	assert.Zero(t, manager.EntryCount())
}

func TestEnvelopeCacheCancellationAfterBuildWakesWaiterWithoutPublishing(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 4), testEnvelopeCacheRSSCharge(t, 8))
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

func TestEnvelopeCacheCloseCancelsAndJoinsPublishedAndBuildingState(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 4), testEnvelopeCacheTotalRSSCharge(t, 4, 4))
	_, err := manager.GetOrBuild(context.Background(), "published", 1, fixedEnvelopeBuild(1))
	require.NoError(t, err)

	started := make(chan *envelopeCacheBuildBuffer, 1)
	buildCanceled := make(chan struct{})
	releaseBuilder := make(chan struct{})
	waiterJoined := make(chan struct{})
	var waiterOnce sync.Once
	manager.testHooks = &envelopeCacheManagerTestHooks{
		waiterJoined: func() {
			waiterOnce.Do(func() { close(waiterJoined) })
		},
	}
	type buildResult struct {
		cache *envelopeCache
		err   error
	}
	results := make(chan buildResult, 2)
	var builds atomic.Int32
	go func() {
		cache, buildErr := manager.GetOrBuild(context.Background(), "building", 1, func(ctx context.Context, buffer *envelopeCacheBuildBuffer) error {
			builds.Add(1)
			started <- buffer
			<-ctx.Done()
			close(buildCanceled)
			<-releaseBuilder
			return ctx.Err()
		})
		results <- buildResult{cache: cache, err: buildErr}
	}()
	buffer := <-started
	require.Positive(t, manager.ReservedBytes())
	go func() {
		cache, buildErr := manager.GetOrBuild(context.Background(), "building", 1, func(context.Context, *envelopeCacheBuildBuffer) error {
			builds.Add(1)
			return nil
		})
		results <- buildResult{cache: cache, err: buildErr}
	}()
	<-waiterJoined

	closeReturned := make(chan struct{})
	go func() {
		manager.Close()
		close(closeReturned)
	}()
	<-buildCanceled
	select {
	case <-closeReturned:
		t.Fatal("Close returned before the active builder exited")
	default:
	}
	select {
	case result := <-results:
		t.Fatalf("waiter returned before the active builder exited: %v", result.err)
	default:
	}
	close(releaseBuilder)

	first := <-results
	second := <-results
	<-closeReturned
	assert.Nil(t, first.cache)
	assert.Nil(t, second.cache)
	assert.ErrorIs(t, first.err, errEnvelopeCacheClosed)
	assert.ErrorIs(t, second.err, errEnvelopeCacheClosed)
	assert.Equal(t, int32(1), builds.Load())
	assert.Nil(t, buffer.entries)
	assert.False(t, buffer.pending.active)
	assert.Zero(t, buffer.pending.reservedBytes)
	assert.Zero(t, manager.TotalBytes())
	assert.Zero(t, manager.ReservedBytes())
	assert.Zero(t, manager.EntryCount())
	manager.mu.Lock()
	assert.Empty(t, manager.builds)
	manager.mu.Unlock()
	cache, err := manager.GetOrBuild(context.Background(), "after-close", 1, fixedEnvelopeBuild(2))
	assert.Nil(t, cache)
	assert.ErrorIs(t, err, errEnvelopeCacheClosed)
}

func TestEnvelopeCacheObjectCountMismatchIsFormatError(t *testing.T) {
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 2), testEnvelopeCacheRSSCharge(t, 4))

	cache, err := manager.GetOrBuild(context.Background(), "points", 2, fixedEnvelopeBuild(1))

	assert.Nil(t, cache)
	assert.True(t, udbxerrors.IsFormatError(err))
	assert.Zero(t, manager.TotalBytes())
	assert.Zero(t, manager.ReservedBytes())
	assert.Zero(t, manager.EntryCount())
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

func testEnvelopeCacheRSSCharge(t *testing.T, capacity int) int64 {
	t.Helper()
	charge, err := envelopeCacheRSSChargeForCapacity(capacity)
	require.NoError(t, err)
	return charge
}

func testEnvelopeCacheTotalRSSCharge(t *testing.T, capacities ...int) int64 {
	t.Helper()
	var total int64
	for _, capacity := range capacities {
		charge := testEnvelopeCacheRSSCharge(t, capacity)
		require.LessOrEqual(t, charge, math.MaxInt64-total)
		total += charge
	}
	return total
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
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 2), testEnvelopeCacheRSSCharge(t, 4))
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
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 2), testEnvelopeCacheRSSCharge(t, 4))
	started := make(chan struct{})
	release := make(chan struct{})
	waiterJoined := make(chan struct{})
	wantErr := stderrors.New("stable build error")
	var once sync.Once
	var waiterOnce sync.Once
	manager.testHooks = &envelopeCacheManagerTestHooks{
		waiterJoined: func() {
			waiterOnce.Do(func() { close(waiterJoined) })
		},
	}
	var builds atomic.Int32
	build := func(context.Context, *envelopeCacheBuildBuffer) error {
		builds.Add(1)
		once.Do(func() { close(started) })
		<-release
		return wantErr
	}

	errs := make(chan error, 2)
	go func() { _, err := manager.GetOrBuild(context.Background(), "points", 1, build); errs <- err }()
	<-started
	go func() { _, err := manager.GetOrBuild(context.Background(), "points", 1, build); errs <- err }()
	<-waiterJoined
	close(release)

	assert.ErrorIs(t, <-errs, wantErr)
	assert.ErrorIs(t, <-errs, wantErr)
	assert.Equal(t, int32(1), builds.Load())
}
