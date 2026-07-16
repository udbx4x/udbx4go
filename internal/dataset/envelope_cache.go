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

type envelopeCacheBuildFunc func(context.Context, *envelopeCacheBuildBuffer) error

type envelopeCacheBuildBuffer struct {
	ctx     context.Context
	manager *EnvelopeCacheManager
	pending *envelopeCacheBuild
	entries []envelopeEntry
}

func (b *envelopeCacheBuildBuffer) Append(entry envelopeEntry) error {
	if b == nil || b.manager == nil || b.pending == nil {
		return fmt.Errorf("envelope cache build buffer is unavailable")
	}
	if err := b.ctx.Err(); err != nil {
		return err
	}
	if len(b.entries) == math.MaxInt {
		return errEnvelopeCacheBudgetExceeded
	}
	requiredCapacity := len(b.entries) + 1
	if requiredCapacity > cap(b.entries) {
		newCapacity, err := b.manager.reserveGrowthCapacity(b.pending, cap(b.entries), requiredCapacity)
		if err != nil {
			return err
		}
		if err := b.ctx.Err(); err != nil {
			return err
		}
		grown := make([]envelopeEntry, len(b.entries), newCapacity)
		copy(grown, b.entries)
		b.entries = grown
	}
	b.entries = append(b.entries, entry)
	return nil
}

type envelopeCacheBuild struct {
	done          chan struct{}
	cache         *envelopeCache
	err           error
	reservedBytes int64
	active        bool
}

type envelopeCacheManagerTestHooks struct {
	beforePublish func(context.Context)
	waiterJoined  func()
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
	testHooks     *envelopeCacheManagerTestHooks
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
		var waiterJoined func()
		if m.testHooks != nil {
			waiterJoined = m.testHooks.waiterJoined
		}
		m.mu.Unlock()
		if waiterJoined != nil {
			waiterJoined()
		}
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
	pending := &envelopeCacheBuild{
		done:          make(chan struct{}),
		reservedBytes: estimatedBytes,
		active:        true,
	}
	m.builds[key] = pending
	m.reservedBytes += estimatedBytes
	m.mu.Unlock()

	buildCtx, cancel := context.WithTimeout(ctx, m.policy.BuildTimeout)
	stopCloseCancel := context.AfterFunc(m.closeCtx, cancel)
	buffer := &envelopeCacheBuildBuffer{
		ctx:     buildCtx,
		manager: m,
		pending: pending,
	}
	buildErr := buildCtx.Err()
	if buildErr == nil {
		buffer.entries = make([]envelopeEntry, 0, objectCount)
		buildErr = build(buildCtx, buffer)
	}
	if buildErr == nil {
		buildErr = buildCtx.Err()
	}

	if buildErr == nil && len(buffer.entries) != objectCount {
		buildErr = fmt.Errorf("envelope cache row count mismatch: got %d, want %d", len(buffer.entries), objectCount)
	}
	if buildErr == nil {
		if buildErr = buildCtx.Err(); buildErr == nil {
			sort.Slice(buffer.entries, func(i, j int) bool { return buffer.entries[i].ID < buffer.entries[j].ID })
			buildErr = buildCtx.Err()
		}
	}
	actualBytes, sizeErr := envelopeCacheCapacityBytes(buffer.entries)
	if buildErr == nil && sizeErr != nil {
		buildErr = errEnvelopeCacheBudgetExceeded
	}
	if buildErr == nil && actualBytes > m.policy.MaxDatasetCacheBytes {
		buildErr = errEnvelopeCacheBudgetExceeded
	}
	if buildErr == nil && m.testHooks != nil && m.testHooks.beforePublish != nil {
		m.testHooks.beforePublish(buildCtx)
	}
	if buildErr == nil {
		buildErr = buildCtx.Err()
	}

	m.mu.Lock()
	pending.active = false
	delete(m.builds, key)
	m.reservedBytes -= pending.reservedBytes
	if m.reservedBytes < 0 {
		m.reservedBytes = 0
	}
	if buildErr == nil {
		buildErr = buildCtx.Err()
	}
	if buildErr == nil && m.closed {
		buildErr = errEnvelopeCacheClosed
	}
	if buildErr == nil && exceedsEnvelopeBudget(m.totalBytes, m.reservedBytes, actualBytes, m.policy.MaxTotalCacheBytes) {
		buildErr = errEnvelopeCacheBudgetExceeded
	}
	if buildErr == nil {
		buildErr = buildCtx.Err()
	}
	if buildErr == nil {
		pending.cache = &envelopeCache{entries: buffer.entries, bytes: actualBytes, complete: true}
		m.caches[key] = pending.cache
		m.totalBytes += actualBytes
	}
	pending.err = buildErr
	close(pending.done)
	m.mu.Unlock()
	stopCloseCancel()
	cancel()

	return pending.cache, pending.err
}

func (m *EnvelopeCacheManager) reserveGrowthCapacity(
	pending *envelopeCacheBuild,
	currentCapacity int,
	requiredCapacity int,
) (int, error) {
	maximumCapacity := maxEnvelopeCacheCapacity(m.policy.MaxDatasetCacheBytes)
	if requiredCapacity <= 0 || requiredCapacity > maximumCapacity {
		return 0, errEnvelopeCacheBudgetExceeded
	}

	desiredCapacity := currentCapacity
	if desiredCapacity == 0 {
		desiredCapacity = 1
	} else if desiredCapacity > math.MaxInt/2 {
		desiredCapacity = math.MaxInt
	} else {
		desiredCapacity *= 2
	}
	if desiredCapacity < requiredCapacity {
		desiredCapacity = requiredCapacity
	}
	if desiredCapacity > maximumCapacity {
		desiredCapacity = maximumCapacity
	}

	if err := m.reserveBuildCapacity(pending, desiredCapacity); err == nil {
		return desiredCapacity, nil
	} else if !stderrors.Is(err, errEnvelopeCacheBudgetExceeded) || desiredCapacity == requiredCapacity {
		return 0, err
	}
	if err := m.reserveBuildCapacity(pending, requiredCapacity); err != nil {
		return 0, err
	}
	return requiredCapacity, nil
}

func (m *EnvelopeCacheManager) reserveBuildCapacity(pending *envelopeCacheBuild, capacity int) error {
	desiredBytes, err := envelopeCacheBytesForCapacity(capacity)
	if err != nil || desiredBytes > m.policy.MaxDatasetCacheBytes {
		return errEnvelopeCacheBudgetExceeded
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errEnvelopeCacheClosed
	}
	if pending == nil || !pending.active {
		return fmt.Errorf("envelope cache build is no longer active")
	}
	if desiredBytes <= pending.reservedBytes {
		return nil
	}
	additionalBytes := desiredBytes - pending.reservedBytes
	if exceedsEnvelopeBudget(m.totalBytes, m.reservedBytes, additionalBytes, m.policy.MaxTotalCacheBytes) {
		return errEnvelopeCacheBudgetExceeded
	}
	m.reservedBytes += additionalBytes
	pending.reservedBytes = desiredBytes
	return nil
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
	return envelopeCacheBytesForCapacity(cap(entries))
}

func envelopeCacheBytesForCapacity(capacity int) (int64, error) {
	if capacity < 0 || uint64(capacity) > uint64(math.MaxInt64)/uint64(envelopeEntryBytes) {
		return 0, errEnvelopeCacheBudgetExceeded
	}
	return int64(capacity) * envelopeEntryBytes, nil
}

func maxEnvelopeCacheCapacity(maximumBytes int64) int {
	if maximumBytes <= 0 {
		return 0
	}
	maximum := maximumBytes / envelopeEntryBytes
	if maximum > int64(math.MaxInt) {
		return math.MaxInt
	}
	return int(maximum)
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
