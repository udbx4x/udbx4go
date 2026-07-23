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
	IDColumn       string
	EnvelopeColumn string
	PayloadColumn  string
	CADTypeColumn  string
	RTreeName      string
	Capability     *types.SpatialQueryCapability
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
	if !supportsSpatialQuery(q.info.Kind) {
		return &detectedSpatialCapability{Capability: &types.SpatialQueryCapability{
			DiagnosticReason: types.SpatialQueryReasonUnsupportedDatasetKind,
		}}, nil
	}

	unavailable := func(detected *detectedSpatialCapability, fallbackAvailable bool) *detectedSpatialCapability {
		if detected == nil {
			detected = &detectedSpatialCapability{}
		}
		detected.Capability = &types.SpatialQueryCapability{
			Supported:         true,
			FallbackAvailable: fallbackAvailable,
			DiagnosticReason:  types.SpatialQueryReasonSpatialIndexUnavailable,
		}
		return detected
	}

	detected, geometryRecord, err := q.detectSpatialColumns(ctx)
	if err != nil {
		return nil, err
	}
	if detected == nil {
		return unavailable(nil, false), nil
	}

	if geometryRecord.SpatialIndexEnabled != 1 {
		return unavailable(detected, true), nil
	}

	rtreeName := spatialRTreeName(q.info.TableName, detected.EnvelopeColumn)
	var physicalRTreeName string
	var definition sql.NullString
	err = q.db.QueryRowContext(
		ctx,
		`SELECT name, sql FROM sqlite_master WHERE type = 'table' AND name = ? COLLATE NOCASE`,
		rtreeName,
	).Scan(&physicalRTreeName, &definition)
	if err == sql.ErrNoRows {
		return unavailable(detected, true), nil
	}
	if err != nil {
		return nil, errors.IOError("failed to inspect spatial index definition", err)
	}
	if !definition.Valid || !rtreeDefinitionPattern.MatchString(definition.String) {
		return unavailable(detected, true), nil
	}

	rtreeColumns, err := sqliteTableInfo(ctx, q.db, physicalRTreeName)
	if err != nil {
		return nil, err
	}
	if !validRTreeColumns(rtreeColumns) {
		return unavailable(detected, true), nil
	}

	detected.RTreeName = physicalRTreeName
	detected.Capability = &types.SpatialQueryCapability{
		Supported:         true,
		RTreeAvailable:    true,
		FallbackAvailable: true,
	}
	return detected, nil
}

func (q *SpatialQuerier) detectSpatialColumns(ctx context.Context) (*detectedSpatialCapability, *system.GeometryColumnsRecord, error) {
	records, err := q.geoColsDao.ListByTableNameContext(ctx, q.info.TableName)
	if err != nil {
		return nil, nil, err
	}
	if len(records) != 1 || !strings.EqualFold(records[0].FTableName, q.info.TableName) {
		return nil, nil, nil
	}

	geometryRecord := records[0]
	idColumn := registeredIDColumn(q.record)
	envelopeColumn, payloadColumn, valid := spatialColumnRoles(q.info.Kind, q.record, geometryRecord)
	if !valid {
		return nil, nil, nil
	}

	tableColumns, err := sqliteTableInfo(ctx, q.db, q.info.TableName)
	if err != nil {
		return nil, nil, err
	}
	physicalIDColumn, idFound := physicalColumnName(tableColumns, idColumn)
	physicalEnvelopeColumn, envelopeFound := physicalColumnName(tableColumns, envelopeColumn)
	physicalPayloadColumn, payloadFound := physicalColumnName(tableColumns, payloadColumn)
	if !idFound || !envelopeFound || !payloadFound {
		return nil, nil, nil
	}
	var physicalCADTypeColumn string
	if q.info.Kind == types.DatasetKindCAD {
		var typeFound bool
		physicalCADTypeColumn, typeFound = physicalColumnName(tableColumns, "SmGeoType")
		if !typeFound {
			return nil, nil, nil
		}
	}

	return &detectedSpatialCapability{
		IDColumn:       physicalIDColumn,
		EnvelopeColumn: physicalEnvelopeColumn,
		PayloadColumn:  physicalPayloadColumn,
		CADTypeColumn:  physicalCADTypeColumn,
	}, geometryRecord, nil
}

func spatialColumnRoles(
	kind types.DatasetKind,
	record *system.SmRegisterRecord,
	geometryRecord *system.GeometryColumnsRecord,
) (string, string, bool) {
	geometryColumn := geometryRecord.FGeometryColumn
	if kind == types.DatasetKindText || kind == types.DatasetKindCAD {
		if !strings.EqualFold(geometryColumn, "SmIndexKey") ||
			geometryRecord.GeometryType != 3 ||
			!validTextCADEnvelopeDimension(kind, geometryRecord.CoordDimension) {
			return "", "", false
		}
		registeredColumn := registeredGeometryColumn(record)
		if registeredColumn != "" && !strings.EqualFold(registeredColumn, "SmGeometry") {
			return "", "", false
		}
		return geometryColumn, "SmGeometry", true
	}

	if geometryColumn == "" {
		return "", "", false
	}
	registeredColumn := registeredGeometryColumn(record)
	if registeredColumn != "" && !strings.EqualFold(registeredColumn, geometryColumn) {
		return "", "", false
	}
	return geometryColumn, geometryColumn, true
}

func validTextCADEnvelopeDimension(kind types.DatasetKind, dimension int) bool {
	if kind == types.DatasetKindCAD {
		return dimension == 2 || dimension == 3
	}
	return dimension == 2
}

func supportsSpatialQuery(kind types.DatasetKind) bool {
	switch kind {
	case types.DatasetKindPoint,
		types.DatasetKindLine,
		types.DatasetKindRegion,
		types.DatasetKindText,
		types.DatasetKindPointZ,
		types.DatasetKindLineZ,
		types.DatasetKindRegionZ,
		types.DatasetKindCAD:
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

func spatialRTreeName(tableName, envelopeColumn string) string {
	return "idx_" + tableName + "_" + envelopeColumn
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
	_, found := physicalColumnName(columns, name)
	return found
}

func physicalColumnName(columns []sqliteColumnInfo, name string) (string, bool) {
	for _, column := range columns {
		if strings.EqualFold(column.name, name) {
			return column.name, true
		}
	}
	return "", false
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
