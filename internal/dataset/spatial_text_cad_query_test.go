package dataset

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/codec"
	"github.com/udbx4x/udbx4go/internal/sqliteutil"
	"github.com/udbx4x/udbx4go/internal/system"
	udbxerrors "github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestSpatialQueryTextEnvelopeCacheLoadsGeoTextPayload(t *testing.T) {
	db, querier := createSpatialTextQueryFixture(t)
	defer db.Close()
	insertSpatialTextFeature(t, db, querier, 1, 10, 20, "inside")

	result, err := querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{MinX: 9, MinY: 19, MaxX: 11, MaxY: 21},
		Limit:  10,
	})

	require.NoError(t, err)
	require.Len(t, result.Features, 1)
	assert.Equal(t, 1, result.Features[0].ID)
	text, ok := result.Features[0].Geometry.(*types.TextGeometry)
	require.True(t, ok)
	assert.Equal(t, "inside", text.Text)
	assert.Len(t, text.BBox, 4)
	assert.Equal(t, types.SpatialQueryStrategyEnvelopeCache, result.Strategy)
}

func TestSpatialQueryTextNullableEnvelopeSemantics(t *testing.T) {
	t.Run("double null is skipped", func(t *testing.T) {
		db, querier := createSpatialTextQueryFixture(t)
		defer db.Close()
		insertRawSpatialTextRow(t, db, querier, 1, nil, nil)

		result, err := querier.Query(context.Background(), types.SpatialQueryOptions{
			Bounds: types.BoundingBox{MinX: -1, MinY: -1, MaxX: 1, MaxY: 1},
			Limit:  10,
		})
		require.NoError(t, err)
		assert.Empty(t, result.Features)
	})

	t.Run("payload without index is unavailable", func(t *testing.T) {
		db, querier := createSpatialTextQueryFixture(t)
		defer db.Close()
		payload, err := codec.NewGeoTextCodec().Encode(&types.TextGeometry{Text: "missing", Anchor: []float64{0, 0}})
		require.NoError(t, err)
		insertRawSpatialTextRow(t, db, querier, 1, payload, nil)

		manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 2), testEnvelopeCacheRSSCharge(t, 4))
		_, err = querier.QueryWithEnvelopeCache(context.Background(), types.SpatialQueryOptions{
			Bounds: types.BoundingBox{MinX: -1, MinY: -1, MaxX: 1, MaxY: 1},
			Limit:  10,
		}, manager)
		assertSpatialQueryError(t, err, types.SpatialQueryReasonSpatialIndexUnavailable, udbxerrors.CodeUnsupported)
		assert.Zero(t, manager.EntryCount())
	})

	t.Run("index without payload is corrupt", func(t *testing.T) {
		db, querier := createSpatialTextQueryFixture(t)
		defer db.Close()
		indexKey, err := codec.EncodeEnvelopeIndexKey([]float64{100, 100, 101, 101}, 4326)
		require.NoError(t, err)
		insertRawSpatialTextRow(t, db, querier, 1, nil, indexKey)

		_, err = querier.Query(context.Background(), types.SpatialQueryOptions{
			Bounds: types.BoundingBox{MinX: -1, MinY: -1, MaxX: 1, MaxY: 1},
			Limit:  10,
		})
		assertSpatialQueryError(t, err, types.SpatialQueryReasonCorruptGeometry, udbxerrors.CodeFormatError)
	})

	t.Run("malformed index is corrupt", func(t *testing.T) {
		db, querier := createSpatialTextQueryFixture(t)
		defer db.Close()
		payload, err := codec.NewGeoTextCodec().Encode(&types.TextGeometry{Text: "broken", Anchor: []float64{0, 0}})
		require.NoError(t, err)
		insertRawSpatialTextRow(t, db, querier, 1, payload, []byte{0x00, 0x01})

		_, err = querier.Query(context.Background(), types.SpatialQueryOptions{
			Bounds: types.BoundingBox{MinX: -1, MinY: -1, MaxX: 1, MaxY: 1},
			Limit:  10,
		})
		assertSpatialQueryError(t, err, types.SpatialQueryReasonCorruptGeometry, udbxerrors.CodeFormatError)
	})
}

func TestSpatialQueryTextAndCADEnvelopeCacheUsesSmallViewportAndStableOrder(t *testing.T) {
	for _, kind := range []types.DatasetKind{types.DatasetKindText, types.DatasetKindCAD} {
		t.Run(kind.String(), func(t *testing.T) {
			fixture := createSpatialTextCADQueryFixture(t, kind, false)
			defer fixture.db.Close()
			fixture.insertFeature(t, 3, 10, 10, "edge-max")
			fixture.insertFeature(t, 1, 0, 0, "edge-min")
			fixture.insertFeature(t, 2, 5, 5, "center")
			fixture.insertFeature(t, 99, 50, 50, "outside")

			result, err := fixture.querier.Query(context.Background(), types.SpatialQueryOptions{
				Bounds: types.BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10},
				Limit:  2,
			})
			require.NoError(t, err)
			assert.Equal(t, []int{1, 2}, spatialFeatureIDs(result.Features))
			assert.Equal(t, types.SpatialQueryStrategyEnvelopeCache, result.Strategy)
			assert.True(t, result.HasMore)
			assertSpatialTextCADGeometry(t, kind, result.Features[0], "edge-min")
			assert.NotContains(t, result.Features[0].Attributes, "SmGeometry")
			assert.NotContains(t, result.Features[0].Attributes, "SmIndexKey")
		})
	}
}

func TestSpatialQueryTextAndCADRTreeMatchesEnvelopeCache(t *testing.T) {
	for _, kind := range []types.DatasetKind{types.DatasetKindText, types.DatasetKindCAD} {
		t.Run(kind.String(), func(t *testing.T) {
			cacheFixture := createSpatialTextCADQueryFixture(t, kind, false)
			defer cacheFixture.db.Close()
			rtreeFixture := createSpatialTextCADQueryFixture(t, kind, true)
			defer rtreeFixture.db.Close()
			for _, feature := range []struct {
				id   int
				x, y float64
				name string
			}{
				{id: 1, x: 1, y: 1, name: "first"},
				{id: 2, x: 5, y: 5, name: "second"},
				{id: 3, x: 20, y: 20, name: "outside"},
			} {
				cacheFixture.insertFeature(t, feature.id, feature.x, feature.y, feature.name)
				rtreeFixture.insertFeature(t, feature.id, feature.x, feature.y, feature.name)
			}
			options := types.SpatialQueryOptions{
				Bounds: types.BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10},
				Limit:  10,
			}

			cacheResult, err := cacheFixture.querier.Query(context.Background(), options)
			require.NoError(t, err)
			rtreeResult, err := rtreeFixture.querier.Query(context.Background(), options)
			require.NoError(t, err)
			assert.Equal(t, spatialFeatureIDs(cacheResult.Features), spatialFeatureIDs(rtreeResult.Features))
			assert.Equal(t, types.SpatialQueryStrategyEnvelopeCache, cacheResult.Strategy)
			assert.Equal(t, types.SpatialQueryStrategyRTree, rtreeResult.Strategy)
		})
	}
}

func TestSpatialQueryTextAndCADRequiredIDsPreserveOrderAndDecode(t *testing.T) {
	for _, kind := range []types.DatasetKind{types.DatasetKindText, types.DatasetKindCAD} {
		t.Run(kind.String(), func(t *testing.T) {
			fixture := createSpatialTextCADQueryFixture(t, kind, false)
			defer fixture.db.Close()
			fixture.insertFeature(t, 1, 1, 1, "first")
			fixture.insertFeature(t, 2, 2, 2, "second")
			fixture.insertFeature(t, 99, 50, 50, "required")

			result, err := fixture.querier.Query(context.Background(), types.SpatialQueryOptions{
				Bounds:      types.BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10},
				Limit:       1,
				RequiredIDs: []int{99, 2, 99},
			})
			require.NoError(t, err)
			assert.Equal(t, []int{1, 99, 2}, spatialFeatureIDs(result.Features))
			assert.True(t, result.HasMore)
			assertSpatialTextCADGeometry(t, kind, result.Features[1], "required")
		})
	}
}

func TestSpatialQueryTextAndCADDecodeOnlyMatchedPayloads(t *testing.T) {
	for _, kind := range []types.DatasetKind{types.DatasetKindText, types.DatasetKindCAD} {
		for _, withRTree := range []bool{false, true} {
			t.Run(kind.String()+fmt.Sprintf("/rtree-%t", withRTree), func(t *testing.T) {
				fixture := createSpatialTextCADQueryFixture(t, kind, withRTree)
				defer fixture.db.Close()
				fixture.insertFeature(t, 1, 1, 1, "inside")
				outsideIndex, err := codec.EncodeEnvelopeIndexKey([]float64{50, 50, 50, 50}, 4326)
				require.NoError(t, err)
				fixture.insertRaw(t, 2, []byte{0x01, 0x02}, outsideIndex, 1)

				result, err := fixture.querier.Query(context.Background(), types.SpatialQueryOptions{
					Bounds: types.BoundingBox{MinX: 0, MinY: 0, MaxX: 2, MaxY: 2},
					Limit:  10,
				})
				require.NoError(t, err)
				assert.Equal(t, []int{1}, spatialFeatureIDs(result.Features))

				_, err = fixture.querier.Query(context.Background(), types.SpatialQueryOptions{
					Bounds: types.BoundingBox{MinX: 49, MinY: 49, MaxX: 51, MaxY: 51},
					Limit:  10,
				})
				assertSpatialQueryError(t, err, types.SpatialQueryReasonCorruptGeometry, udbxerrors.CodeFormatError)
			})
		}
	}
}

func TestSpatialQueryTextAndCADFilterEnvelopeButDecodePayload(t *testing.T) {
	for _, kind := range []types.DatasetKind{types.DatasetKindText, types.DatasetKindCAD} {
		for _, withRTree := range []bool{false, true} {
			t.Run(kind.String()+fmt.Sprintf("/rtree-%t", withRTree), func(t *testing.T) {
				fixture := createSpatialTextCADQueryFixture(t, kind, withRTree)
				defer fixture.db.Close()
				var payload []byte
				var geoType int
				var err error
				if kind == types.DatasetKindText {
					payload, err = codec.NewGeoTextCodec().Encode(&types.TextGeometry{Text: "payload", Anchor: []float64{50, 50}})
					geoType = 7
				} else {
					payload, err = codec.NewCadGeometryCodec().Encode(&types.CadPointGeometry{XCoord: 50, YCoord: 50})
					geoType = 1
				}
				require.NoError(t, err)
				fixture.insertRaw(t, 1, payload, mustSpatialEnvelopeIndex(t, 1, 1), geoType)

				result, err := fixture.querier.Query(context.Background(), types.SpatialQueryOptions{
					Bounds: types.BoundingBox{MinX: 0, MinY: 0, MaxX: 2, MaxY: 2},
					Limit:  10,
				})
				require.NoError(t, err)
				require.Len(t, result.Features, 1)
				if kind == types.DatasetKindText {
					geometry := result.Features[0].Geometry.(*types.TextGeometry)
					assert.Equal(t, []float64{50, 50}, geometry.Anchor)
					assert.Equal(t, []float64{1, 1, 1, 1}, geometry.BBox)
				} else {
					geometry := result.Features[0].Geometry.(*types.CadPointGeometry)
					assert.Equal(t, 50.0, geometry.XCoord)
					assert.Equal(t, 50.0, geometry.YCoord)
				}
			})
		}
	}
}

func TestSpatialQueryTextAndCADCancellationReturnsTimeout(t *testing.T) {
	for _, kind := range []types.DatasetKind{types.DatasetKindText, types.DatasetKindCAD} {
		t.Run(kind.String(), func(t *testing.T) {
			fixture := createSpatialTextCADQueryFixture(t, kind, false)
			defer fixture.db.Close()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, err := fixture.querier.Query(ctx, types.SpatialQueryOptions{
				Bounds: types.BoundingBox{},
				Limit:  10,
			})
			assertSpatialQueryError(t, err, types.SpatialQueryReasonQueryTimeout, udbxerrors.CodeUdbxError)
		})
	}
}

func TestSpatialQueryCADNullableEnvelopeSemantics(t *testing.T) {
	tests := []struct {
		name       string
		payload    interface{}
		indexKey   interface{}
		wantReason types.SpatialQueryReason
		wantCode   string
	}{
		{name: "double null"},
		{name: "missing index", payload: []byte{0x01}, wantReason: types.SpatialQueryReasonSpatialIndexUnavailable, wantCode: udbxerrors.CodeUnsupported},
		{name: "orphan index", indexKey: mustSpatialEnvelopeIndex(t, 100, 100), wantReason: types.SpatialQueryReasonCorruptGeometry, wantCode: udbxerrors.CodeFormatError},
		{name: "malformed index", payload: []byte{0x01}, indexKey: []byte{0x00, 0x01}, wantReason: types.SpatialQueryReasonCorruptGeometry, wantCode: udbxerrors.CodeFormatError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := createSpatialTextCADQueryFixture(t, types.DatasetKindCAD, false)
			defer fixture.db.Close()
			fixture.insertRaw(t, 1, tt.payload, tt.indexKey, 1)

			result, err := fixture.querier.Query(context.Background(), types.SpatialQueryOptions{
				Bounds: types.BoundingBox{MinX: -1, MinY: -1, MaxX: 1, MaxY: 1},
				Limit:  10,
			})
			if tt.wantReason == "" {
				require.NoError(t, err)
				assert.Empty(t, result.Features)
				return
			}
			assertSpatialQueryError(t, err, tt.wantReason, tt.wantCode)
		})
	}
}

func TestTextBuildFeatureUsesNullableIndexEnvelopeAndRequiresIndexColumn(t *testing.T) {
	db, querier := createSpatialTextQueryFixture(t)
	defer db.Close()
	geometry := &types.TextGeometry{Text: "label", Anchor: []float64{1, 2}}
	payload, err := codec.NewGeoTextCodec().Encode(geometry)
	require.NoError(t, err)
	dataset := NewTextDataset(db, querier.info)

	feature, err := dataset.buildFeature(
		[]string{"SmID", "SmGeometry", "SmIndexKey", "SmUserID", "name"},
		[]interface{}{int64(1), payload, nil, int64(0), "label"},
	)
	require.NoError(t, err)
	assert.Empty(t, feature.Geometry.(*types.TextGeometry).BBox)
	assert.Equal(t, map[string]interface{}{"name": "label"}, feature.Attributes)

	_, err = dataset.buildFeature(
		[]string{"SmID", "SmGeometry"},
		[]interface{}{int64(1), payload},
	)
	require.Error(t, err)
}

func TestSpatialQueryTextUsesDetectedPhysicalColumnsWithQuotedUnicodeTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	tableName := `标注"Layer`
	idColumn := `sMiD"编号`
	payloadColumn := "sMgEoMeTrY"
	envelopeColumn := "sMiNdExKeY"
	quotedTable, err := sqliteutil.QuoteIdentifier(tableName)
	require.NoError(t, err)
	quotedID, err := sqliteutil.QuoteIdentifier(idColumn)
	require.NoError(t, err)
	quotedPayload, err := sqliteutil.QuoteIdentifier(payloadColumn)
	require.NoError(t, err)
	quotedEnvelope, err := sqliteutil.QuoteIdentifier(envelopeColumn)
	require.NoError(t, err)
	_, err = db.Exec(fmt.Sprintf(
		"CREATE TABLE %s (%s INTEGER PRIMARY KEY, SmUserID INTEGER, %s BLOB, %s POLYGON, name TEXT)",
		quotedTable, quotedID, quotedPayload, quotedEnvelope,
	))
	require.NoError(t, err)
	record := &system.SmRegisterRecord{
		SmDatasetID:   1,
		SmDatasetName: "quoted",
		SmTableName:   tableName,
		SmDatasetType: int(types.DatasetKindText),
		SmObjectCount: 1,
		SmIDColName:   sql.NullString{String: `smid"编号`, Valid: true},
		SmGeoColName:  sql.NullString{String: "smgeometry", Valid: true},
		SmSRID:        sql.NullInt32{Int32: 4326, Valid: true},
	}
	require.NoError(t, system.NewGeometryColumnsDao(db).Insert(&system.GeometryColumnsRecord{
		FTableName:          `标注"layer`,
		FGeometryColumn:     "smindexkey",
		GeometryType:        3,
		CoordDimension:      2,
		SRID:                3857,
		SpatialIndexEnabled: 0,
	}))
	geometry := &types.TextGeometry{Text: "quoted", Anchor: []float64{3, 4}}
	payload, err := codec.NewGeoTextCodec().Encode(geometry)
	require.NoError(t, err)
	indexKey, err := codec.EncodeTextIndexKey(geometry, 3857)
	require.NoError(t, err)
	_, err = db.Exec(fmt.Sprintf(
		"INSERT INTO %s (%s, SmUserID, %s, %s, name) VALUES (?, 0, ?, ?, ?)",
		quotedTable, quotedID, quotedPayload, quotedEnvelope,
	), 7, payload, indexKey, "quoted")
	require.NoError(t, err)

	result, err := NewSpatialQuerier(db, record.ToDatasetInfo(), record).Query(
		context.Background(),
		types.SpatialQueryOptions{
			Bounds: types.BoundingBox{MinX: 2, MinY: 3, MaxX: 4, MaxY: 5},
			Limit:  10,
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Features, 1)
	assert.Equal(t, 7, result.Features[0].ID)
	assert.Equal(t, "quoted", result.Features[0].Geometry.(*types.TextGeometry).Text)
}

func TestSpatialQueryTextAndCADRealSampleSmallViewportSmoke(t *testing.T) {
	path, err := filepath.Abs(sampleDataSpatialCapabilityPath(t))
	require.NoError(t, err)
	dsn := url.URL{Scheme: "file", Path: path}
	query := dsn.Query()
	query.Set("mode", "ro")
	dsn.RawQuery = query.Encode()
	db, err := sql.Open("sqlite3", dsn.String())
	require.NoError(t, err)
	defer db.Close()

	records, err := system.NewSmRegisterDao(db).ListAll()
	require.NoError(t, err)
	wanted := map[string]types.DatasetKind{
		"County_T": types.DatasetKindText,
		"CADDT":    types.DatasetKindCAD,
	}
	found := make(map[string]bool)
	for _, record := range records {
		kind, ok := wanted[record.SmDatasetName]
		if !ok {
			continue
		}
		found[record.SmDatasetName] = true
		querier := NewSpatialQuerier(db, record.ToDatasetInfo(), record)
		detected, err := querier.detectCapability(context.Background())
		require.NoError(t, err)
		require.True(t, detected.Capability.FallbackAvailable)
		quotedTable, err := sqliteutil.QuoteIdentifier(record.SmTableName)
		require.NoError(t, err)
		quotedEnvelope, err := sqliteutil.QuoteIdentifier(detected.EnvelopeColumn)
		require.NoError(t, err)
		quotedPayload, err := sqliteutil.QuoteIdentifier(detected.PayloadColumn)
		require.NoError(t, err)
		var indexKey []byte
		err = db.QueryRow(
			"SELECT " + quotedEnvelope + " FROM " + quotedTable +
				" WHERE " + quotedPayload + " IS NOT NULL AND " + quotedEnvelope + " IS NOT NULL LIMIT 1",
		).Scan(&indexKey)
		require.NoError(t, err)
		envelope, err := codec.ReadGaiaEnvelope(indexKey)
		require.NoError(t, err)

		result, err := querier.Query(context.Background(), types.SpatialQueryOptions{
			Bounds: envelope,
			Limit:  1,
		})
		require.NoError(t, err)
		require.NotEmpty(t, result.Features)
		if kind == types.DatasetKindText {
			assert.IsType(t, &types.TextGeometry{}, result.Features[0].Geometry)
		} else {
			_, ok := result.Features[0].Geometry.(types.CadGeometry)
			assert.True(t, ok)
		}
	}
	assert.True(t, found["County_T"])
	assert.True(t, found["CADDT"])
}

type spatialTextCADQueryFixture struct {
	db        *sql.DB
	querier   *SpatialQuerier
	kind      types.DatasetKind
	tableName string
	withRTree bool
}

func createSpatialTextCADQueryFixture(
	t *testing.T,
	kind types.DatasetKind,
	withRTree bool,
) *spatialTextCADQueryFixture {
	t.Helper()
	db := setupTestDB(t)
	tableName := "labels"
	columns := `SmID INTEGER PRIMARY KEY,
		SmUserID INTEGER DEFAULT 0,
		SmGeometry BLOB,
		SmIndexKey POLYGON,
		name TEXT`
	if kind == types.DatasetKindCAD {
		tableName = "cad_layers"
		columns = `SmID INTEGER PRIMARY KEY,
			SmUserID INTEGER DEFAULT 0,
			SmGeoType INTEGER,
			SmGeometry BLOB,
			SmIndexKey POLYGON,
			name TEXT`
	}
	_, err := db.Exec(fmt.Sprintf("CREATE TABLE %s (%s)", tableName, columns))
	require.NoError(t, err)
	spatialIndexEnabled := 0
	if withRTree {
		spatialIndexEnabled = 1
	}
	record := &system.SmRegisterRecord{
		SmDatasetID:   1,
		SmDatasetName: tableName,
		SmTableName:   tableName,
		SmDatasetType: int(kind),
		SmIDColName:   sql.NullString{String: "SmID", Valid: true},
		SmGeoColName:  sql.NullString{String: "SmGeometry", Valid: true},
		SmSRID:        sql.NullInt32{Int32: 4326, Valid: true},
	}
	require.NoError(t, system.NewGeometryColumnsDao(db).Insert(&system.GeometryColumnsRecord{
		FTableName:          tableName,
		FGeometryColumn:     "SmIndexKey",
		GeometryType:        3,
		CoordDimension:      2,
		SRID:                4326,
		SpatialIndexEnabled: spatialIndexEnabled,
	}))
	if withRTree {
		createSpatialRTree(t, db, tableName, "SmIndexKey", "pkid, xmin, xmax, ymin, ymax")
	}
	return &spatialTextCADQueryFixture{
		db:        db,
		querier:   NewSpatialQuerier(db, record.ToDatasetInfo(), record),
		kind:      kind,
		tableName: tableName,
		withRTree: withRTree,
	}
}

func (f *spatialTextCADQueryFixture) insertFeature(
	t *testing.T,
	id int,
	x float64,
	y float64,
	name string,
) {
	t.Helper()
	var payload []byte
	var geoType int
	var err error
	if f.kind == types.DatasetKindText {
		geometry := &types.TextGeometry{Text: name, Anchor: []float64{x, y}}
		payload, err = codec.NewGeoTextCodec().Encode(geometry)
		geoType = 7
	} else {
		payload, err = codec.NewCadGeometryCodec().Encode(&types.CadPointGeometry{XCoord: x, YCoord: y})
		geoType = 1
	}
	require.NoError(t, err)
	indexKey := mustSpatialEnvelopeIndex(t, x, y)
	f.insertRaw(t, id, payload, indexKey, geoType)
}

func (f *spatialTextCADQueryFixture) insertRaw(
	t *testing.T,
	id int,
	payload interface{},
	indexKey interface{},
	geoType int,
) {
	t.Helper()
	var err error
	if f.kind == types.DatasetKindText {
		_, err = f.db.Exec(
			`INSERT INTO labels (SmID, SmUserID, SmGeometry, SmIndexKey, name) VALUES (?, 0, ?, ?, ?)`,
			id, payload, indexKey, fmt.Sprintf("feature-%d", id),
		)
	} else {
		_, err = f.db.Exec(
			`INSERT INTO cad_layers (SmID, SmUserID, SmGeoType, SmGeometry, SmIndexKey, name) VALUES (?, 0, ?, ?, ?, ?)`,
			id, geoType, payload, indexKey, fmt.Sprintf("feature-%d", id),
		)
	}
	require.NoError(t, err)
	if f.withRTree && indexKey != nil {
		blob, ok := indexKey.([]byte)
		require.True(t, ok)
		envelope, err := codec.ReadGaiaEnvelope(blob)
		require.NoError(t, err)
		_, err = f.db.Exec(
			"INSERT INTO "+mustQuoteSpatialIdentifier(t, spatialRTreeName(f.tableName, "SmIndexKey"))+
				" (pkid, xmin, xmax, ymin, ymax) VALUES (?, ?, ?, ?, ?)",
			id, envelope.MinX, envelope.MaxX, envelope.MinY, envelope.MaxY,
		)
		require.NoError(t, err)
	}
	var count int
	require.NoError(t, f.db.QueryRow("SELECT COUNT(*) FROM "+mustQuoteSpatialIdentifier(t, f.tableName)).Scan(&count))
	f.querier.info.ObjectCount = count
	f.querier.record.SmObjectCount = count
}

func mustSpatialEnvelopeIndex(t *testing.T, x, y float64) []byte {
	t.Helper()
	indexKey, err := codec.EncodeEnvelopeIndexKey([]float64{x, y, x, y}, 4326)
	require.NoError(t, err)
	return indexKey
}

func assertSpatialTextCADGeometry(
	t *testing.T,
	kind types.DatasetKind,
	feature *types.Feature,
	wantText string,
) {
	t.Helper()
	if kind == types.DatasetKindText {
		geometry, ok := feature.Geometry.(*types.TextGeometry)
		require.True(t, ok)
		assert.Equal(t, wantText, geometry.Text)
		assert.Len(t, geometry.BBox, 4)
		return
	}
	_, ok := feature.Geometry.(*types.CadPointGeometry)
	require.True(t, ok)
}

func createSpatialTextQueryFixture(t *testing.T) (*sql.DB, *SpatialQuerier) {
	t.Helper()
	db := setupTestDB(t)
	_, err := db.Exec(`CREATE TABLE labels (
		SmID INTEGER PRIMARY KEY,
		SmUserID INTEGER DEFAULT 0,
		SmGeometry BLOB,
		SmIndexKey POLYGON,
		name TEXT
	)`)
	require.NoError(t, err)
	record := &system.SmRegisterRecord{
		SmDatasetID:   1,
		SmDatasetName: "labels",
		SmTableName:   "labels",
		SmDatasetType: int(types.DatasetKindText),
		SmIDColName:   sql.NullString{String: "SmID", Valid: true},
		SmGeoColName:  sql.NullString{String: "SmGeometry", Valid: true},
		SmSRID:        sql.NullInt32{Int32: 4326, Valid: true},
	}
	require.NoError(t, system.NewGeometryColumnsDao(db).Insert(&system.GeometryColumnsRecord{
		FTableName:          "labels",
		FGeometryColumn:     "SmIndexKey",
		GeometryType:        3,
		CoordDimension:      2,
		SRID:                4326,
		SpatialIndexEnabled: 0,
	}))
	return db, NewSpatialQuerier(db, record.ToDatasetInfo(), record)
}

func insertSpatialTextFeature(
	t *testing.T,
	db *sql.DB,
	querier *SpatialQuerier,
	id int,
	x float64,
	y float64,
	text string,
) {
	t.Helper()
	geometry := &types.TextGeometry{Text: text, Anchor: []float64{x, y}}
	payload, err := codec.NewGeoTextCodec().Encode(geometry)
	require.NoError(t, err)
	indexKey, err := codec.EncodeTextIndexKey(geometry, 4326)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO labels (SmID, SmUserID, SmGeometry, SmIndexKey, name) VALUES (?, 0, ?, ?, ?)`,
		id, payload, indexKey, text,
	)
	require.NoError(t, err)
	updateSpatialQueryFixtureCount(t, db, querier)
}

func insertRawSpatialTextRow(
	t *testing.T,
	db *sql.DB,
	querier *SpatialQuerier,
	id int,
	payload interface{},
	indexKey interface{},
) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO labels (SmID, SmUserID, SmGeometry, SmIndexKey, name) VALUES (?, 0, ?, ?, 'raw')`,
		id, payload, indexKey,
	)
	require.NoError(t, err)
	updateSpatialQueryFixtureCount(t, db, querier)
}

func updateSpatialQueryFixtureCount(t *testing.T, db *sql.DB, querier *SpatialQuerier) {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM labels`).Scan(&count))
	querier.info.ObjectCount = count
	querier.record.SmObjectCount = count
}
