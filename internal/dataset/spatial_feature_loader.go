package dataset

import (
	"context"
	"database/sql"
	"strings"

	"github.com/udbx4x/udbx4go/internal/sqliteutil"
	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

const spatialFeatureIDBatchSize = 500

type spatialFeatureBuilder func(columns []string, values []interface{}) (*types.Feature, error)

func loadSpatialFeaturesByIDs(
	ctx context.Context,
	db *sql.DB,
	tableName string,
	idColumn string,
	ids []int,
	buildFeature spatialFeatureBuilder,
) (map[int]*types.Feature, error) {
	features := make(map[int]*types.Feature, len(ids))
	for start := 0; start < len(ids); start += spatialFeatureIDBatchSize {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		end := start + spatialFeatureIDBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch, err := loadSpatialFeatureBatch(ctx, db, tableName, idColumn, ids[start:end], buildFeature)
		if err != nil {
			return nil, err
		}
		for id, feature := range batch {
			features[id] = feature
		}
	}
	return features, nil
}

func loadSpatialFeatureBatch(
	ctx context.Context,
	db *sql.DB,
	tableName string,
	idColumn string,
	ids []int,
	buildFeature spatialFeatureBuilder,
) (features map[int]*types.Feature, returnErr error) {
	features = make(map[int]*types.Feature, len(ids))
	if len(ids) == 0 {
		return features, nil
	}

	quotedTable, err := sqliteutil.QuoteIdentifier(tableName)
	if err != nil {
		return nil, errors.IOError("failed to quote dataset table name", err)
	}
	quotedID, err := sqliteutil.QuoteIdentifier(idColumn)
	if err != nil {
		return nil, errors.IOError("failed to quote feature ID column", err)
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for index, id := range ids {
		placeholders[index] = "?"
		args[index] = id
	}
	query := "SELECT * FROM " + quotedTable +
		" WHERE " + quotedID + " IN (" + strings.Join(placeholders, ", ") + ")" +
		" ORDER BY " + quotedID
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, errors.IOError("failed to load spatial query features", err)
	}
	defer func() {
		if closeErr := rows.Close(); returnErr == nil && closeErr != nil {
			returnErr = errors.IOError("failed to close spatial query features", closeErr)
		}
	}()

	columns, err := rows.Columns()
	if err != nil {
		return nil, errors.IOError("failed to get spatial query columns", err)
	}
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		values := make([]interface{}, len(columns))
		valuePointers := make([]interface{}, len(columns))
		for index := range values {
			valuePointers[index] = &values[index]
		}
		if err := rows.Scan(valuePointers...); err != nil {
			return nil, errors.IOError("failed to scan spatial query feature", err)
		}
		feature, err := buildFeature(columns, values)
		if err != nil {
			return nil, err
		}
		if feature == nil {
			continue
		}
		features[feature.ID] = feature
	}
	if err := rows.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, errors.IOError("error iterating spatial query features", err)
	}
	return features, nil
}

func spatialFeatureRowIsDoubleNull(
	columns []string,
	values []interface{},
	payloadColumn string,
	envelopeColumn string,
) bool {
	payloadNull, payloadFound := false, false
	envelopeNull, envelopeFound := false, false
	for index, column := range columns {
		switch {
		case strings.EqualFold(column, payloadColumn):
			payloadFound = true
			payloadNull = values[index] == nil
		case strings.EqualFold(column, envelopeColumn):
			envelopeFound = true
			envelopeNull = values[index] == nil
		}
	}
	return payloadFound && envelopeFound && payloadNull && envelopeNull
}
