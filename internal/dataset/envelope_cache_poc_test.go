package dataset

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/codec"
	"github.com/udbx4x/udbx4go/pkg/types"
)

type envelopeCachePOCFixture struct {
	db *sql.DB
}

type pocEnvelopeEntry struct {
	id   int64
	minX float64
	minY float64
	maxX float64
	maxY float64
}

var envelopeCachePOCSizes = []int{10_000, 50_000, 100_000, 250_000, 500_000}

func newEnvelopeCachePOCFixture(t testing.TB, size int) *envelopeCachePOCFixture {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "envelope-cache-poc.db")
	require.NoError(t, createEnvelopeCachePOCFixture(dbPath, size))
	db, err := openEnvelopeCachePOCFixtureReadOnly(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	return &envelopeCachePOCFixture{db: db}
}

func createEnvelopeCachePOCFixture(dbPath string, size int) (returnErr error) {
	if size <= 0 {
		return errors.New("PoC fixture size must be positive")
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); returnErr == nil && closeErr != nil {
			returnErr = closeErr
		}
	}()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	_, err = db.Exec(`CREATE TABLE poc_points (SmID INTEGER PRIMARY KEY, SmGeometry BLOB NOT NULL)`)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO poc_points (SmID, SmGeometry) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	pointCodec := codec.NewGaiaPointCodec()
	for id := 1; id <= size; id++ {
		xCoordinate := float64((id - 1) % 1000)
		yCoordinate := float64((id - 1) / 1000)
		point := &types.PointGeometry{Coordinates: []float64{xCoordinate, yCoordinate}}
		geometry, encodeErr := pointCodec.EncodePoint(point, 4326)
		if encodeErr != nil {
			return encodeErr
		}
		_, err = stmt.Exec(id, geometry)
		if err != nil {
			return err
		}
	}
	if err := stmt.Close(); err != nil {
		return err
	}
	return tx.Commit()
}

func openEnvelopeCachePOCFixtureReadOnly(dbPath string) (*sql.DB, error) {
	dsn := url.URL{Scheme: "file", Path: dbPath}
	query := dsn.Query()
	query.Set("mode", "ro")
	query.Set("immutable", "1")
	dsn.RawQuery = query.Encode()

	db, err := sql.Open("sqlite3", dsn.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func TestEnvelopeCachePOCFixtureHasNoSpatialIndex(t *testing.T) {
	fixture := newEnvelopeCachePOCFixture(t, 1)

	var indexCount int
	err := fixture.db.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE name = 'idx_poc_points_SmGeometry'
	`).Scan(&indexCount)
	require.NoError(t, err)
	assert.Zero(t, indexCount)

	var header []byte
	err = fixture.db.QueryRow(`
		SELECT substr(SmGeometry, 1, 43)
		FROM poc_points
		WHERE SmID = 1
	`).Scan(&header)
	require.NoError(t, err)
	assert.Len(t, header, 43)
}

func TestEnvelopeCachePOCCorePaths(t *testing.T) {
	const size = 1_000
	dbPath := filepath.Join(t.TempDir(), "external-envelope-cache-poc.db")
	require.NoError(t, createEnvelopeCachePOCFixture(dbPath, size))

	db, err := openEnvelopeCachePOCFixtureReadOnly(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	cache, err := buildPOCEnvelopeCache(context.Background(), db, size, nil)
	require.NoError(t, err)
	require.Len(t, cache, size)
	assert.Equal(t, uint64(cap(cache))*uint64(unsafe.Sizeof(pocEnvelopeEntry{})), pocEnvelopeCacheBytes(cache))

	candidateIDs := filterPOCEnvelopeCache(cache)
	require.Len(t, candidateIDs, size/100)

	loadIDs := make([]int64, 600)
	for i := range loadIDs {
		loadIDs[i] = int64(i + 1)
	}
	decoded, err := loadPOCCandidateGeometries(context.Background(), db, loadIDs)
	require.NoError(t, err)
	assert.Equal(t, len(loadIDs), decoded)

	ctx, cancel := context.WithCancel(context.Background())
	canceledCache, err := buildPOCEnvelopeCache(ctx, db, size, func(scanned int) {
		if scanned >= size/10 {
			cancel()
		}
	})
	cancel()
	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, canceledCache)
	assert.True(t, assertPOCCancelResourcesReleased(t, db))
}

func TestEnvelopeCachePOCGenerateFixture(t *testing.T) {
	dbPath := os.Getenv("UDBX_ENVELOPE_CACHE_POC_GENERATE_PATH")
	if dbPath == "" {
		t.Skip("fixture generation environment is not configured")
	}
	size, err := strconv.Atoi(os.Getenv("UDBX_ENVELOPE_CACHE_POC_SIZE"))
	require.NoError(t, err)
	require.Contains(t, envelopeCachePOCSizes, size)
	require.NoError(t, createEnvelopeCachePOCFixture(dbPath, size))
}

func BenchmarkEnvelopeCachePOC(b *testing.B) {
	dbPath := os.Getenv("UDBX_ENVELOPE_CACHE_POC_FIXTURE_PATH")
	sizeText := os.Getenv("UDBX_ENVELOPE_CACHE_POC_SIZE")
	if dbPath == "" || sizeText == "" {
		b.Skip("set UDBX_ENVELOPE_CACHE_POC_FIXTURE_PATH and UDBX_ENVELOPE_CACHE_POC_SIZE")
	}
	size, err := strconv.Atoi(sizeText)
	if err != nil {
		b.Fatal(err)
	}
	if !containsEnvelopeCachePOCSize(size) {
		b.Fatalf("unsupported PoC size: %d", size)
	}

	db, err := openEnvelopeCachePOCFixtureReadOnly(dbPath)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := db.Close(); err != nil {
			b.Error(err)
		}
	})

	var cache []pocEnvelopeEntry
	b.Run(strconv.Itoa(size)+"/build/cold", func(b *testing.B) {
		cache = benchmarkEnvelopeCachePOCBuild(b, db, size, true)
	})
	b.Run(strconv.Itoa(size)+"/build/hot", func(b *testing.B) {
		_ = benchmarkEnvelopeCachePOCBuild(b, db, size, false)
	})

	var candidateIDs []int64
	b.Run(strconv.Itoa(size)+"/filter", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			candidateIDs = filterPOCEnvelopeCache(cache)
		}
		b.StopTimer()
		if len(candidateIDs) != size/100 {
			b.Fatalf("candidate count: got %d, want %d", len(candidateIDs), size/100)
		}
	})

	b.Run(strconv.Itoa(size)+"/load", func(b *testing.B) {
		b.ReportAllocs()
		var decoded int
		var loadErr error
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			decoded, loadErr = loadPOCCandidateGeometries(context.Background(), db, candidateIDs)
			if loadErr != nil {
				b.Fatal(loadErr)
			}
		}
		b.StopTimer()
		if decoded != len(candidateIDs) {
			b.Fatalf("decoded count: got %d, want %d", decoded, len(candidateIDs))
		}
	})

	b.Run(strconv.Itoa(size)+"/cancel", func(b *testing.B) {
		b.ReportAllocs()
		var canceledCache []pocEnvelopeEntry
		var cancelErr error
		b.StopTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithCancel(context.Background())
			b.StartTimer()
			canceledCache, cancelErr = buildPOCEnvelopeCache(ctx, db, size, func(scanned int) {
				if scanned >= size/10 {
					cancel()
				}
			})
			b.StopTimer()
			cancel()
			if cancelErr != nil && !errors.Is(cancelErr, context.Canceled) {
				b.Fatal(cancelErr)
			}
		}
		if !errors.Is(cancelErr, context.Canceled) {
			b.Fatalf("cancel error: got %v, want context.Canceled", cancelErr)
		}
		if canceledCache != nil {
			b.Fatal("canceled build published a partial cache")
		}
		if !assertPOCCancelResourcesReleased(b, db) {
			b.Fatal("SQLite resources were not released after cancellation")
		}
		b.ReportMetric(1, "cancel_released")
	})
	runtime.KeepAlive(cache)
}

func containsEnvelopeCachePOCSize(size int) bool {
	for _, candidate := range envelopeCachePOCSizes {
		if candidate == size {
			return true
		}
	}
	return false
}

func benchmarkEnvelopeCachePOCBuild(b *testing.B, db *sql.DB, size int, reportStableRSS bool) []pocEnvelopeEntry {
	b.Helper()
	b.ReportAllocs()
	runtime.GC()
	debug.FreeOSMemory()
	baselineRSS := stablePOCRSSBytes(b)

	var cache []pocEnvelopeEntry
	var buildErr error
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache, buildErr = buildPOCEnvelopeCache(context.Background(), db, size, nil)
		if buildErr != nil {
			b.Fatal(buildErr)
		}
	}
	b.StopTimer()
	if len(cache) != size {
		b.Fatalf("cache size: got %d, want %d", len(cache), size)
	}

	cacheBytes := pocEnvelopeCacheBytes(cache)
	b.ReportMetric(float64(cacheBytes)/(1024*1024), "cache_mib")
	if reportStableRSS {
		runtime.GC()
		debug.FreeOSMemory()
		stableRSS := stablePOCRSSBytes(b)
		rssDelta := uint64(0)
		if stableRSS > baselineRSS {
			rssDelta = stableRSS - baselineRSS
		}
		b.ReportMetric(float64(rssDelta)/(1024*1024), "stable_rss_delta_mib")
	}
	runtime.KeepAlive(cache)
	return cache
}

func buildPOCEnvelopeCache(
	ctx context.Context,
	db *sql.DB,
	expectedSize int,
	onScanned func(int),
) ([]pocEnvelopeEntry, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT SmID, substr(SmGeometry, 1, 43)
		FROM poc_points
		ORDER BY SmID
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]pocEnvelopeEntry, 0, expectedSize)
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var id int64
		var headerBytes []byte
		if err := rows.Scan(&id, &headerBytes); err != nil {
			return nil, err
		}

		header, err := codec.ReadGaiaHeader(headerBytes)
		if err != nil {
			return nil, err
		}
		for _, value := range header.MBR {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, errors.New("non-finite GAIA MBR in PoC fixture")
			}
		}

		entries = append(entries, pocEnvelopeEntry{
			id:   id,
			minX: header.MBR[0],
			minY: header.MBR[1],
			maxX: header.MBR[2],
			maxY: header.MBR[3],
		})
		if onScanned != nil {
			onScanned(len(entries))
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, err
	}
	if len(entries) != expectedSize {
		return nil, errors.New("PoC envelope cache row count mismatch")
	}

	return entries, nil
}

func filterPOCEnvelopeCache(cache []pocEnvelopeEntry) []int64 {
	candidateIDs := make([]int64, 0, len(cache)/100)
	for _, entry := range cache {
		if entry.maxX >= 0 && entry.minX <= 9 && entry.maxY >= -1 && entry.minY <= 1_000_000 {
			candidateIDs = append(candidateIDs, entry.id)
		}
	}
	return candidateIDs
}

func loadPOCCandidateGeometries(ctx context.Context, db *sql.DB, candidateIDs []int64) (int, error) {
	const batchSize = 500
	geometryCodec := codec.NewGaiaGeometryCodec()
	decoded := 0

	for start := 0; start < len(candidateIDs); start += batchSize {
		end := start + batchSize
		if end > len(candidateIDs) {
			end = len(candidateIDs)
		}
		batch := candidateIDs[start:end]
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")
		query := `SELECT SmGeometry FROM poc_points WHERE SmID IN (` + placeholders + `) ORDER BY SmID`
		args := make([]any, len(batch))
		for i, id := range batch {
			args[i] = id
		}

		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return 0, err
		}
		for rows.Next() {
			var geometryBytes []byte
			if err := rows.Scan(&geometryBytes); err != nil {
				rows.Close()
				return 0, err
			}
			geometry, err := geometryCodec.Decode(geometryBytes)
			if err != nil {
				rows.Close()
				return 0, err
			}
			decoded++
			runtime.KeepAlive(geometry)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return 0, err
		}
		if err := rows.Close(); err != nil {
			return 0, err
		}
	}

	return decoded, nil
}

func assertPOCCancelResourcesReleased(t testing.TB, db *sql.DB) bool {
	t.Helper()
	runtime.GC()
	debug.FreeOSMemory()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var one int
	if err := db.QueryRowContext(ctx, `SELECT 1`).Scan(&one); err != nil {
		return false
	}
	return one == 1 && db.Stats().InUse == 0
}

func currentPOCRSSBytes(t testing.TB) uint64 {
	t.Helper()
	output, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(os.Getpid())).Output()
	require.NoError(t, err)
	rssKiB, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	require.NoError(t, err)
	return rssKiB * 1024
}

func stablePOCRSSBytes(t testing.TB) uint64 {
	t.Helper()
	samples := make([]uint64, 0, 3)
	for i := 0; i < 3; i++ {
		samples = append(samples, currentPOCRSSBytes(t))
		if i < 2 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return samples[len(samples)/2]
}

func pocEnvelopeCacheBytes(entries []pocEnvelopeEntry) uint64 {
	return uint64(cap(entries)) * uint64(unsafe.Sizeof(pocEnvelopeEntry{}))
}
