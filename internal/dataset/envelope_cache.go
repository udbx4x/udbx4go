package dataset

import (
	"context"
	stderrors "errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	udbxerrors "github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

var (
	errEnvelopeCacheBudgetExceeded = stderrors.New("envelope cache budget exceeded")
	errEnvelopeCacheClosed         = stderrors.New("envelope cache manager is closed")
	errEnvelopeCacheInvalidContext = stderrors.New("envelope cache build context is unavailable")
	errEnvelopeCacheRowOverflow    = stderrors.New("envelope cache scanned row count overflow")
)

type envelopeEntry struct {
	ID         int64
	MinX, MinY float64
	MaxX, MaxY float64
}

const (
	// The 250k-to-500k PoC P95 slope is about 80.3 resident bytes per
	// capacity entry. The rounded 80-byte charge plus a roughly 4 MiB fixed
	// charge models stable RSS retained by the Go runtime and SQLite path.
	// These are empirical resource-policy charges, not object-count limits or
	// the in-memory size of envelopeEntry.
	envelopeCacheFixedRSSChargeBytes       int64 = 4 * 1024 * 1024
	envelopeCacheRSSChargePerCapacityEntry int64 = 80
)

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
	ctx         context.Context
	manager     *EnvelopeCacheManager
	pending     *envelopeCacheBuild
	entries     []envelopeEntry
	scannedRows int
}

func (b *envelopeCacheBuildBuffer) Append(entry envelopeEntry) error {
	if err := b.validateRowAction(); err != nil {
		return err
	}
	if len(b.entries) == math.MaxInt {
		return errEnvelopeCacheBudgetExceeded
	}
	requiredCapacity := len(b.entries) + 1
	if requiredCapacity > cap(b.entries) {
		oldCapacity := cap(b.entries)
		newCapacity, err := b.manager.reserveGrowthCapacity(b.pending, oldCapacity, requiredCapacity)
		if err != nil {
			return err
		}
		if err := b.ctx.Err(); err != nil {
			return err
		}
		grown := make([]envelopeEntry, len(b.entries), newCapacity)
		copy(grown, b.entries)
		b.entries = grown
		if err := b.manager.commitGrowthCapacity(b.pending, oldCapacity, newCapacity); err != nil {
			return err
		}
	}
	if err := b.validateRowAction(); err != nil {
		return err
	}
	b.entries = append(b.entries, entry)
	b.scannedRows++
	return nil
}

func (b *envelopeCacheBuildBuffer) SkipRow() error {
	if err := b.validateRowAction(); err != nil {
		return err
	}
	b.scannedRows++
	return nil
}

func (b *envelopeCacheBuildBuffer) validateRowAction() error {
	if b == nil || b.manager == nil || b.pending == nil {
		return fmt.Errorf("envelope cache build buffer is unavailable")
	}
	if b.ctx == nil {
		return errEnvelopeCacheInvalidContext
	}
	if err := b.ctx.Err(); err != nil {
		return err
	}
	if b.scannedRows == math.MaxInt {
		return errEnvelopeCacheRowOverflow
	}

	b.manager.mu.Lock()
	defer b.manager.mu.Unlock()
	if b.pending.invalidated {
		return context.Canceled
	}
	if !b.pending.active {
		return fmt.Errorf("envelope cache build is no longer active")
	}
	return nil
}

type envelopeCacheBuild struct {
	done          chan struct{}
	cache         *envelopeCache
	err           error
	reservedBytes int64
	active        bool
	invalidated   bool
	cancel        context.CancelFunc
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
	activeBuilds  map[*envelopeCacheBuild]struct{}
	totalBytes    int64
	reservedBytes int64
	closeCtx      context.Context
	cancel        context.CancelFunc
	closeDone     chan struct{}
	closed        bool
	testHooks     *envelopeCacheManagerTestHooks
}

func NewEnvelopeCacheManager(policy types.SpatialQueryPolicy) (*EnvelopeCacheManager, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	closeCtx, cancel := context.WithCancel(context.Background())
	return &EnvelopeCacheManager{
		policy:       policy,
		caches:       make(map[string]*envelopeCache),
		builds:       make(map[string]*envelopeCacheBuild),
		activeBuilds: make(map[*envelopeCacheBuild]struct{}),
		closeCtx:     closeCtx,
		cancel:       cancel,
		closeDone:    make(chan struct{}),
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
	if ctx == nil {
		return nil, errEnvelopeCacheInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if build == nil {
		return nil, fmt.Errorf("envelope cache build function is required")
	}
	estimatedBytes, err := envelopeCacheRSSChargeForCapacity(objectCount)
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
	buildCtx, cancel := context.WithTimeout(ctx, m.policy.BuildTimeout)
	stopCloseCancel := context.AfterFunc(m.closeCtx, cancel)
	pending := &envelopeCacheBuild{
		done:          make(chan struct{}),
		reservedBytes: estimatedBytes,
		active:        true,
		cancel:        cancel,
	}
	m.builds[key] = pending
	m.activeBuilds[pending] = struct{}{}
	m.reservedBytes += estimatedBytes
	m.mu.Unlock()

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

	if buildErr == nil && buffer.scannedRows != objectCount {
		buildErr = udbxerrors.FormatError(
			"envelope cache row count does not match dataset metadata",
			fmt.Errorf("scanned %d rows, metadata declares %d", buffer.scannedRows, objectCount),
		)
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
	delete(m.activeBuilds, pending)
	isCurrentGeneration := m.builds[key] == pending
	if isCurrentGeneration {
		delete(m.builds, key)
	}
	m.reservedBytes -= pending.reservedBytes
	pending.reservedBytes = 0
	if m.closed {
		buildErr = errEnvelopeCacheClosed
	} else if pending.invalidated || !isCurrentGeneration {
		buildErr = context.Canceled
	} else if buildErr == nil {
		buildErr = buildCtx.Err()
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
	buffer.entries = nil
	pending.err = buildErr
	close(pending.done)
	m.mu.Unlock()
	stopCloseCancel()
	cancel()

	return pending.cache, pending.err
}

// InvalidateDataset removes every cache generation owned by tableName.
func (m *EnvelopeCacheManager) InvalidateDataset(tableName string) {
	if m == nil {
		return
	}
	prefix := tableName + "\x00"
	cancels := make([]context.CancelFunc, 0)

	m.mu.Lock()
	for key, cache := range m.caches {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		delete(m.caches, key)
		m.totalBytes -= cache.bytes
	}
	for key, pending := range m.builds {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		delete(m.builds, key)
		pending.invalidated = true
		if pending.cancel != nil {
			cancels = append(cancels, pending.cancel)
		}
	}
	m.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
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

	if err := m.reserveBuildGrowth(pending, currentCapacity, desiredCapacity); err == nil {
		return desiredCapacity, nil
	} else if !stderrors.Is(err, errEnvelopeCacheBudgetExceeded) || desiredCapacity == requiredCapacity {
		return 0, err
	}
	if err := m.reserveBuildGrowth(pending, currentCapacity, requiredCapacity); err != nil {
		return 0, err
	}
	return requiredCapacity, nil
}

func (m *EnvelopeCacheManager) reserveBuildGrowth(
	pending *envelopeCacheBuild,
	currentCapacity int,
	desiredCapacity int,
) error {
	currentBytes, currentErr := envelopeCacheRSSChargeForCapacity(currentCapacity)
	peakBytes, peakErr := envelopeCacheRSSPeakCharge(currentCapacity, desiredCapacity)
	if currentErr != nil || peakErr != nil || peakBytes > m.policy.MaxDatasetCacheBytes {
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
	if pending.reservedBytes != currentBytes {
		return fmt.Errorf("envelope cache build reservation does not match current capacity")
	}
	additionalBytes := peakBytes - pending.reservedBytes
	if exceedsEnvelopeBudget(m.totalBytes, m.reservedBytes, additionalBytes, m.policy.MaxTotalCacheBytes) {
		return errEnvelopeCacheBudgetExceeded
	}
	m.reservedBytes += additionalBytes
	pending.reservedBytes = peakBytes
	return nil
}

func (m *EnvelopeCacheManager) commitGrowthCapacity(
	pending *envelopeCacheBuild,
	previousCapacity int,
	currentCapacity int,
) error {
	previousBytes, previousErr := envelopeCacheRSSVariableChargeForCapacity(previousCapacity)
	currentBytes, currentErr := envelopeCacheRSSChargeForCapacity(currentCapacity)
	peakBytes, peakErr := envelopeCacheRSSPeakCharge(previousCapacity, currentCapacity)
	if previousErr != nil || currentErr != nil || peakErr != nil {
		return errEnvelopeCacheBudgetExceeded
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if pending == nil || !pending.active || pending.reservedBytes != peakBytes {
		return fmt.Errorf("envelope cache growth reservation is no longer active")
	}
	m.reservedBytes -= previousBytes
	pending.reservedBytes = currentBytes
	if m.closed {
		return errEnvelopeCacheClosed
	}
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
		closeDone := m.closeDone
		m.mu.Unlock()
		<-closeDone
		return
	}
	m.closed = true
	buildsDone := make([]<-chan struct{}, 0, len(m.activeBuilds))
	for pending := range m.activeBuilds {
		buildsDone = append(buildsDone, pending.done)
	}
	m.mu.Unlock()

	m.cancel()
	for _, done := range buildsDone {
		<-done
	}

	m.mu.Lock()
	m.caches = make(map[string]*envelopeCache)
	m.builds = make(map[string]*envelopeCacheBuild)
	m.activeBuilds = make(map[*envelopeCacheBuild]struct{})
	m.totalBytes = 0
	m.reservedBytes = 0
	close(m.closeDone)
	m.mu.Unlock()
}

func envelopeCacheCapacityBytes(entries []envelopeEntry) (int64, error) {
	return envelopeCacheRSSChargeForCapacity(cap(entries))
}

func envelopeCacheRSSChargeForCapacity(capacity int) (int64, error) {
	variable, err := envelopeCacheRSSVariableChargeForCapacity(capacity)
	if err != nil || variable > math.MaxInt64-envelopeCacheFixedRSSChargeBytes {
		return 0, errEnvelopeCacheBudgetExceeded
	}
	return envelopeCacheFixedRSSChargeBytes + variable, nil
}

func envelopeCacheRSSVariableChargeForCapacity(capacity int) (int64, error) {
	if capacity < 0 || uint64(capacity) > uint64(math.MaxInt64)/uint64(envelopeCacheRSSChargePerCapacityEntry) {
		return 0, errEnvelopeCacheBudgetExceeded
	}
	return int64(capacity) * envelopeCacheRSSChargePerCapacityEntry, nil
}

func envelopeCacheRSSPeakCharge(currentCapacity, desiredCapacity int) (int64, error) {
	current, currentErr := envelopeCacheRSSVariableChargeForCapacity(currentCapacity)
	desired, desiredErr := envelopeCacheRSSVariableChargeForCapacity(desiredCapacity)
	variable, addErr := addEnvelopeCacheBytes(current, desired)
	if currentErr != nil || desiredErr != nil || addErr != nil || variable > math.MaxInt64-envelopeCacheFixedRSSChargeBytes {
		return 0, errEnvelopeCacheBudgetExceeded
	}
	return envelopeCacheFixedRSSChargeBytes + variable, nil
}

func addEnvelopeCacheBytes(left, right int64) (int64, error) {
	if left < 0 || right < 0 || left > math.MaxInt64-right {
		return 0, errEnvelopeCacheBudgetExceeded
	}
	return left + right, nil
}

func maxEnvelopeCacheCapacity(maximumBytes int64) int {
	if maximumBytes < envelopeCacheFixedRSSChargeBytes {
		return 0
	}
	maximum := (maximumBytes - envelopeCacheFixedRSSChargeBytes) / envelopeCacheRSSChargePerCapacityEntry
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
