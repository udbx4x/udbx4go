package dataset

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
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
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	_, err = db.Exec(`CREATE TABLE poc_points (SmID INTEGER PRIMARY KEY, SmGeometry BLOB NOT NULL)`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	stmt, err := tx.Prepare(`INSERT INTO poc_points (SmID, SmGeometry) VALUES (?, ?)`)
	require.NoError(t, err)
	pointCodec := codec.NewGaiaPointCodec()
	for id := 1; id <= size; id++ {
		xCoordinate := float64((id - 1) % 1000)
		yCoordinate := float64((id - 1) / 1000)
		point := &types.PointGeometry{Coordinates: []float64{xCoordinate, yCoordinate}}
		geometry, encodeErr := pointCodec.EncodePoint(point, 4326)
		require.NoError(t, encodeErr)
		_, err = stmt.Exec(id, geometry)
		require.NoError(t, err)
	}
	require.NoError(t, stmt.Close())
	require.NoError(t, tx.Commit())
	require.NoError(t, db.Close())

	db, err = sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	require.NoError(t, db.Ping())

	return &envelopeCachePOCFixture{db: db}
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

func BenchmarkEnvelopeCachePOC(b *testing.B) {
	for _, size := range envelopeCachePOCSizes {
		size := size
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			fixture := newEnvelopeCachePOCFixture(b, size)

			b.Run("build/cold", func(b *testing.B) {
				benchmarkEnvelopeCachePOCBuild(b, fixture.db, size)
			})
			b.Run("build/hot", func(b *testing.B) {
				benchmarkEnvelopeCachePOCBuild(b, fixture.db, size)
			})

			cache, err := buildPOCEnvelopeCache(context.Background(), fixture.db, size, nil)
			require.NoError(b, err)
			require.Len(b, cache, size)

			b.Run("filter", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					candidateIDs := filterPOCEnvelopeCache(cache)
					require.Len(b, candidateIDs, size/100)
					runtime.KeepAlive(candidateIDs)
				}
			})

			candidateIDs := filterPOCEnvelopeCache(cache)
			b.Run("load", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					decoded, loadErr := loadPOCCandidateGeometries(context.Background(), fixture.db, candidateIDs)
					require.NoError(b, loadErr)
					require.Equal(b, len(candidateIDs), decoded)
				}
			})

			b.Run("cancel", func(b *testing.B) {
				b.ReportAllocs()
				b.StopTimer()
				for i := 0; i < b.N; i++ {
					ctx, cancel := context.WithCancel(context.Background())
					b.StartTimer()
					cache, cancelErr := buildPOCEnvelopeCache(ctx, fixture.db, size, func(scanned int) {
						if scanned >= size/10 {
							cancel()
						}
					})
					b.StopTimer()
					cancel()

					require.ErrorIs(b, cancelErr, context.Canceled)
					require.Nil(b, cache)
					require.True(b, assertPOCCancelResourcesReleased(b, fixture.db))
					b.ReportMetric(1, "cancel_released")
				}
			})
		})
	}
}

func benchmarkEnvelopeCachePOCBuild(b *testing.B, db *sql.DB, size int) {
	b.Helper()
	b.ReportAllocs()
	runtime.GC()
	baselineRSS := currentPOCRSSBytes(b)

	b.ResetTimer()
	var cache []pocEnvelopeEntry
	for i := 0; i < b.N; i++ {
		var err error
		cache, err = buildPOCEnvelopeCache(context.Background(), db, size, nil)
		require.NoError(b, err)
		require.Len(b, cache, size)
		runtime.KeepAlive(cache)
	}
	b.StopTimer()

	afterRSS := currentPOCRSSBytes(b)
	rssDelta := uint64(0)
	if afterRSS > baselineRSS {
		rssDelta = afterRSS - baselineRSS
	}
	cacheBytes := pocEnvelopeCacheBytes(cache)
	b.ReportMetric(float64(cacheBytes)/(1024*1024), "cache_mib")
	b.ReportMetric(float64(rssDelta)/(1024*1024), "rss_delta_mib")
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

func pocEnvelopeCacheBytes(entries []pocEnvelopeEntry) uint64 {
	return uint64(cap(entries)) * uint64(unsafe.Sizeof(pocEnvelopeEntry{}))
}
