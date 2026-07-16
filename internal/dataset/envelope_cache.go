package dataset

import (
	"context"
	stderrors "errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"unsafe"

	"github.com/udbx4x/udbx4go/pkg/types"
)

var (
	errEnvelopeCacheBudgetExceeded = stderrors.New("envelope cache budget exceeded")
	errEnvelopeCacheClosed         = stderrors.New("envelope cache manager is closed")
)

type envelopeEntry struct {
	ID         int64
	MinX, MinY float64
	MaxX, MaxY float64
}

const envelopeEntryBytes = int64(unsafe.Sizeof(envelopeEntry{}))

type envelopeCache struct {
	entries  []envelopeEntry
	bytes    int64
	complete bool
}

func (c *envelopeCache) Complete() bool {
	return c != nil && c.complete
}

func (c *envelopeCache) CandidateIDs(bounds types.BoundingBox, limit int) ([]int, bool, error) {
	if c == nil || !c.complete {
		return nil, false, fmt.Errorf("envelope cache is incomplete")
	}
	if err := bounds.Validate(); err != nil {
		return nil, false, err
	}
	if limit <= 0 || limit == math.MaxInt {
		return nil, false, fmt.Errorf("envelope cache candidate limit must allow limit + 1")
	}

	ids := make([]int, 0, initialCandidateCapacity(limit))
	for _, entry := range c.entries {
		if entry.MaxX < bounds.MinX || entry.MinX > bounds.MaxX ||
			entry.MaxY < bounds.MinY || entry.MinY > bounds.MaxY {
			continue
		}
		id := int(entry.ID)
		if id <= 0 || int64(id) != entry.ID {
			return nil, false, fmt.Errorf("envelope cache feature ID %d is outside the supported integer range", entry.ID)
		}
		ids = append(ids, id)
		if len(ids) == limit+1 {
			return ids[:limit], true, nil
		}
	}
	return ids, false, nil
}

type envelopeCacheBuildFunc func(context.Context) ([]envelopeEntry, error)

type envelopeCacheBuild struct {
	done  chan struct{}
	cache *envelopeCache
	err   error
}

// EnvelopeCacheManager owns all envelope caches for one DataSource.
type EnvelopeCacheManager struct {
	mu            sync.Mutex
	policy        types.SpatialQueryPolicy
	caches        map[string]*envelopeCache
	builds        map[string]*envelopeCacheBuild
	totalBytes    int64
	reservedBytes int64
	closeCtx      context.Context
	cancel        context.CancelFunc
	closed        bool
}

func NewEnvelopeCacheManager(policy types.SpatialQueryPolicy) (*EnvelopeCacheManager, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	closeCtx, cancel := context.WithCancel(context.Background())
	return &EnvelopeCacheManager{
		policy:   policy,
		caches:   make(map[string]*envelopeCache),
		builds:   make(map[string]*envelopeCacheBuild),
		closeCtx: closeCtx,
		cancel:   cancel,
	}, nil
}

func (m *EnvelopeCacheManager) GetOrBuild(
	ctx context.Context,
	key string,
	objectCount int,
	build envelopeCacheBuildFunc,
) (*envelopeCache, error) {
	if m == nil {
		return nil, errEnvelopeCacheClosed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if build == nil {
		return nil, fmt.Errorf("envelope cache build function is required")
	}
	estimatedBytes, err := estimateEnvelopeCacheBytes(objectCount)
	if err != nil || estimatedBytes > m.policy.MaxDatasetCacheBytes {
		return nil, errEnvelopeCacheBudgetExceeded
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, errEnvelopeCacheClosed
	}
	if cache := m.caches[key]; cache != nil {
		m.mu.Unlock()
		return cache, nil
	}
	if pending := m.builds[key]; pending != nil {
		m.mu.Unlock()
		select {
		case <-pending.done:
			return pending.cache, pending.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if exceedsEnvelopeBudget(m.totalBytes, m.reservedBytes, estimatedBytes, m.policy.MaxTotalCacheBytes) {
		m.mu.Unlock()
		return nil, errEnvelopeCacheBudgetExceeded
	}
	pending := &envelopeCacheBuild{done: make(chan struct{})}
	m.builds[key] = pending
	m.reservedBytes += estimatedBytes
	m.mu.Unlock()

	buildCtx, cancel := context.WithTimeout(ctx, m.policy.BuildTimeout)
	stopCloseCancel := context.AfterFunc(m.closeCtx, cancel)
	entries, buildErr := build(buildCtx)
	if buildErr == nil {
		buildErr = buildCtx.Err()
	}
	stopCloseCancel()
	cancel()

	if buildErr == nil && len(entries) != objectCount {
		buildErr = fmt.Errorf("envelope cache row count mismatch: got %d, want %d", len(entries), objectCount)
	}
	if buildErr == nil {
		sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	}
	actualBytes, sizeErr := envelopeCacheCapacityBytes(entries)
	if buildErr == nil && sizeErr != nil {
		buildErr = errEnvelopeCacheBudgetExceeded
	}
	if buildErr == nil && actualBytes > m.policy.MaxDatasetCacheBytes {
		buildErr = errEnvelopeCacheBudgetExceeded
	}

	m.mu.Lock()
	delete(m.builds, key)
	m.reservedBytes -= estimatedBytes
	if m.reservedBytes < 0 {
		m.reservedBytes = 0
	}
	if buildErr == nil && m.closed {
		buildErr = errEnvelopeCacheClosed
	}
	if buildErr == nil && exceedsEnvelopeBudget(m.totalBytes, m.reservedBytes, actualBytes, m.policy.MaxTotalCacheBytes) {
		buildErr = errEnvelopeCacheBudgetExceeded
	}
	if buildErr == nil {
		pending.cache = &envelopeCache{entries: entries, bytes: actualBytes, complete: true}
		m.caches[key] = pending.cache
		m.totalBytes += actualBytes
	}
	pending.err = buildErr
	close(pending.done)
	m.mu.Unlock()

	return pending.cache, pending.err
}

func (m *EnvelopeCacheManager) TotalBytes() int64 {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.totalBytes
}

func (m *EnvelopeCacheManager) ReservedBytes() int64 {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reservedBytes
}

func (m *EnvelopeCacheManager) EntryCount() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.caches)
}

func (m *EnvelopeCacheManager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	m.caches = make(map[string]*envelopeCache)
	m.totalBytes = 0
	m.reservedBytes = 0
	m.cancel()
	m.mu.Unlock()
}

func estimateEnvelopeCacheBytes(objectCount int) (int64, error) {
	if objectCount < 0 || uint64(objectCount) > uint64(math.MaxInt64)/uint64(envelopeEntryBytes) {
		return 0, errEnvelopeCacheBudgetExceeded
	}
	return int64(objectCount) * envelopeEntryBytes, nil
}

func envelopeCacheCapacityBytes(entries []envelopeEntry) (int64, error) {
	if uint64(cap(entries)) > uint64(math.MaxInt64)/uint64(envelopeEntryBytes) {
		return 0, errEnvelopeCacheBudgetExceeded
	}
	return int64(cap(entries)) * envelopeEntryBytes, nil
}

func exceedsEnvelopeBudget(committed, reserved, requested, maximum int64) bool {
	if committed < 0 || reserved < 0 || requested < 0 || maximum < 0 {
		return true
	}
	if committed > maximum-reserved {
		return true
	}
	return requested > maximum-committed-reserved
}
