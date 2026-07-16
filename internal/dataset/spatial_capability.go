package dataset

import (
	"context"
	"database/sql"
	"regexp"
	"strings"

	"github.com/udbx4x/udbx4go/internal/sqliteutil"
	"github.com/udbx4x/udbx4go/internal/system"
	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

var rtreeDefinitionPattern = regexp.MustCompile(`(?is)^\s*CREATE\s+VIRTUAL\s+TABLE\b[\s\S]*\bUSING\s+rtree\s*\(`)

// SpatialQuerier owns spatial metadata detection and query execution for one dataset.
type SpatialQuerier struct {
	db         *sql.DB
	info       *types.DatasetInfo
	record     *system.SmRegisterRecord
	geoColsDao *system.GeometryColumnsDao
}

type detectedSpatialCapability struct {
	Capability     *types.SpatialQueryCapability
	RTreeName      string
	GeometryColumn string
	IDColumn       string
}

type sqliteColumnInfo struct {
	name     string
	typeName string
}

// NewSpatialQuerier creates a spatial querier from public and raw UDBX metadata.
func NewSpatialQuerier(db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) *SpatialQuerier {
	return &SpatialQuerier{
		db:         db,
		info:       info,
		record:     record,
		geoColsDao: system.NewGeometryColumnsDao(db),
	}
}

// Capability reports whether the Task 4 RTree path is available.
func (q *SpatialQuerier) Capability(ctx context.Context) (*types.SpatialQueryCapability, error) {
	detected, err := q.detectCapability(ctx)
	if err != nil {
		return nil, err
	}
	return detected.Capability, nil
}

func (q *SpatialQuerier) detectCapability(ctx context.Context) (*detectedSpatialCapability, error) {
	if !supportsRTreeSpatialQuery(q.info.Kind) {
		return &detectedSpatialCapability{Capability: &types.SpatialQueryCapability{
			DiagnosticReason: types.SpatialQueryReasonUnsupportedDatasetKind,
		}}, nil
	}

	unavailable := func() *detectedSpatialCapability {
		return &detectedSpatialCapability{Capability: &types.SpatialQueryCapability{
			Supported:         true,
			FallbackAvailable: true,
			DiagnosticReason:  types.SpatialQueryReasonSpatialIndexUnavailable,
		}}
	}

	records, err := q.geoColsDao.ListByTableNameContext(ctx, q.info.TableName)
	if err != nil {
		return nil, err
	}
	if len(records) != 1 || !strings.EqualFold(records[0].FTableName, q.info.TableName) {
		return unavailable(), nil
	}

	geometryRecord := records[0]
	geometryColumn := geometryRecord.FGeometryColumn
	if geometryColumn == "" {
		return unavailable(), nil
	}
	if registeredGeometryColumn(q.record) != "" && !strings.EqualFold(registeredGeometryColumn(q.record), geometryColumn) {
		return unavailable(), nil
	}

	tableColumns, err := sqliteTableInfo(ctx, q.db, q.info.TableName)
	if err != nil {
		return nil, err
	}
	if !hasColumn(tableColumns, geometryColumn) {
		return unavailable(), nil
	}

	if geometryRecord.SpatialIndexEnabled != 1 {
		return unavailable(), nil
	}

	idColumn := registeredIDColumn(q.record)
	if !hasColumn(tableColumns, idColumn) {
		return unavailable(), nil
	}

	rtreeName := spatialRTreeName(q.info.TableName, geometryColumn)
	var physicalRTreeName string
	var definition sql.NullString
	err = q.db.QueryRowContext(
		ctx,
		`SELECT name, sql FROM sqlite_master WHERE type = 'table' AND name = ? COLLATE NOCASE`,
		rtreeName,
	).Scan(&physicalRTreeName, &definition)
	if err == sql.ErrNoRows {
		return unavailable(), nil
	}
	if err != nil {
		return nil, errors.IOError("failed to inspect spatial index definition", err)
	}
	if !definition.Valid || !rtreeDefinitionPattern.MatchString(definition.String) {
		return unavailable(), nil
	}

	rtreeColumns, err := sqliteTableInfo(ctx, q.db, physicalRTreeName)
	if err != nil {
		return nil, err
	}
	if !validRTreeColumns(rtreeColumns) {
		return unavailable(), nil
	}

	return &detectedSpatialCapability{
		Capability: &types.SpatialQueryCapability{
			Supported:         true,
			RTreeAvailable:    true,
			FallbackAvailable: true,
		},
		RTreeName:      physicalRTreeName,
		GeometryColumn: geometryColumn,
		IDColumn:       idColumn,
	}, nil
}

func supportsRTreeSpatialQuery(kind types.DatasetKind) bool {
	switch kind {
	case types.DatasetKindPoint,
		types.DatasetKindLine,
		types.DatasetKindRegion,
		types.DatasetKindPointZ,
		types.DatasetKindLineZ,
		types.DatasetKindRegionZ:
		return true
	default:
		return false
	}
}

func registeredGeometryColumn(record *system.SmRegisterRecord) string {
	if record != nil && record.SmGeoColName.Valid {
		return record.SmGeoColName.String
	}
	return ""
}

func registeredIDColumn(record *system.SmRegisterRecord) string {
	if record != nil && record.SmIDColName.Valid {
		if record.SmIDColName.String != "" {
			return record.SmIDColName.String
		}
	}
	return "SmID"
}

func spatialRTreeName(tableName, geometryColumn string) string {
	return "idx_" + tableName + "_" + geometryColumn
}

func sqliteTableInfo(ctx context.Context, db *sql.DB, tableName string) ([]sqliteColumnInfo, error) {
	quotedTable, err := sqliteutil.QuoteIdentifier(tableName)
	if err != nil {
		return nil, errors.IOError("failed to quote SQLite table name", err)
	}
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+quotedTable+")")
	if err != nil {
		return nil, errors.IOError("failed to inspect SQLite table columns", err)
	}
	defer rows.Close()

	var columns []sqliteColumnInfo
	for rows.Next() {
		var cid int
		var column sqliteColumnInfo
		var notNull int
		var defaultValue interface{}
		var primaryKey int
		if err := rows.Scan(&cid, &column.name, &column.typeName, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, errors.IOError("failed to scan SQLite table column", err)
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.IOError("error iterating SQLite table columns", err)
	}
	return columns, nil
}

func hasColumn(columns []sqliteColumnInfo, name string) bool {
	for _, column := range columns {
		if strings.EqualFold(column.name, name) {
			return true
		}
	}
	return false
}

func validRTreeColumns(columns []sqliteColumnInfo) bool {
	requiredTypes := map[string]func(string) bool{
		"pkid": func(typeName string) bool { return strings.Contains(strings.ToUpper(typeName), "INT") },
		"xmin": isSQLiteRealType,
		"xmax": isSQLiteRealType,
		"ymin": isSQLiteRealType,
		"ymax": isSQLiteRealType,
	}
	found := make(map[string]bool, len(requiredTypes))
	for _, column := range columns {
		name := strings.ToLower(column.name)
		validateType, required := requiredTypes[name]
		if required && validateType(column.typeName) {
			found[name] = true
		}
	}
	for name := range requiredTypes {
		if !found[name] {
			return false
		}
	}
	return true
}

func isSQLiteRealType(typeName string) bool {
	upper := strings.ToUpper(typeName)
	return strings.Contains(upper, "REAL") ||
		strings.Contains(upper, "FLOA") ||
		strings.Contains(upper, "DOUB")
}
