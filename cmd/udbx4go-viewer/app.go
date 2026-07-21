package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	udbx4go "github.com/udbx4x/udbx4go"
	udbxerrors "github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	pageSize                            = 100
	defaultSpatialPreviewLimit          = 1000
	defaultSpatialVertexBudget          = 1000000
	spatialPreviewStrategyBoundedSample = "bounded_sample"
)

// App struct
type App struct {
	ctx                   context.Context
	dataSourceLifecycleMu sync.Mutex
	dataSourceMu          sync.Mutex
	dataSource            *udbx4go.DataSource
	dataSourceQueries     *sync.WaitGroup
	dataSourceContext     context.Context
	dataSourceCancel      context.CancelFunc
	fileGeneration        uint64
	currentPath           string
	currentDatasetCount   int
	settingsPathOverride  string
	benchmarkConfigPath   string
	benchmarkConfig       *BenchmarkConfigDTO
	previewQueryHook      func(context.Context) error
	previewResultHook     func(context.Context)
}

// DatasetInfoDTO represents dataset information for the frontend
type DatasetInfoDTO struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	ObjectCount int    `json:"objectCount"`
	IconType    string `json:"iconType"`
}

// PageData represents a page of dataset records
type PageData struct {
	Rows        [][]string `json:"rows"`
	Columns     []string   `json:"columns"`
	CurrentPage int        `json:"currentPage"`
	TotalPages  int        `json:"totalPages"`
}

// BoundingBoxDTO represents an axis-aligned spatial extent.
type BoundingBoxDTO struct {
	MinX float64 `json:"minX"`
	MinY float64 `json:"minY"`
	MaxX float64 `json:"maxX"`
	MaxY float64 `json:"maxY"`
}

// SpatialSummaryDTO describes whether and how a dataset can be previewed.
type SpatialSummaryDTO struct {
	DatasetName            string          `json:"datasetName"`
	Kind                   string          `json:"kind"`
	SRID                   *int            `json:"srid,omitempty"`
	Extent                 *BoundingBoxDTO `json:"extent,omitempty"`
	ObjectCount            int             `json:"objectCount"`
	EstimatedVertexCount   int             `json:"estimatedVertexCount"`
	PreviewSupported       bool            `json:"previewSupported"`
	UnsupportedReason      string          `json:"unsupportedReason,omitempty"`
	ViewportQuerySupported bool            `json:"viewportQuerySupported"`
	RTreeAvailable         bool            `json:"rtreeAvailable"`
	QueryDiagnosticReason  string          `json:"queryDiagnosticReason,omitempty"`
}

// SpatialPreviewRequestDTO limits the amount of geometry sent to the frontend.
type SpatialPreviewRequestDTO struct {
	Viewport    *BoundingBoxDTO `json:"viewport,omitempty"`
	Limit       int             `json:"limit"`
	MaxVertices int             `json:"maxVertices"`
	Simplify    bool            `json:"simplify"`
	RequiredIDs []int           `json:"requiredIds,omitempty"`
}

// PreviewGeometryDTO is a renderer-neutral geometry payload for PoC adapters.
type PreviewGeometryDTO struct {
	Type        string        `json:"type"`
	Coordinates []interface{} `json:"coordinates"`
	HasZ        bool          `json:"hasZ"`
}

// PreviewFeatureDTO is the minimal spatial feature contract for table-map linking.
type PreviewFeatureDTO struct {
	ID         int                `json:"id"`
	Geometry   PreviewGeometryDTO `json:"geometry"`
	BBox       *BoundingBoxDTO    `json:"bbox,omitempty"`
	Properties map[string]string  `json:"properties,omitempty"`
}

// SpatialPreviewDTO is a bounded preview response for a spatial dataset.
type SpatialPreviewDTO struct {
	DatasetName          string              `json:"datasetName"`
	Kind                 string              `json:"kind"`
	SRID                 *int                `json:"srid,omitempty"`
	Extent               *BoundingBoxDTO     `json:"extent,omitempty"`
	Features             []PreviewFeatureDTO `json:"features"`
	EstimatedVertexCount int                 `json:"estimatedVertexCount"`
	Sampled              bool                `json:"sampled"`
	SampleReason         string              `json:"sampleReason,omitempty"`
	QueriedBounds        *BoundingBoxDTO     `json:"queriedBounds,omitempty"`
	Strategy             string              `json:"strategy"`
	HasMore              bool                `json:"hasMore"`
	DegradedReason       string              `json:"degradedReason,omitempty"`
	QueryDurationMS      float64             `json:"queryDurationMs"`
	FileGeneration       uint64              `json:"fileGeneration"`
}

// FeatureAttributesDTO is returned when the user identifies a feature.
type FeatureAttributesDTO struct {
	DatasetName  string            `json:"datasetName"`
	ID           int               `json:"id"`
	GeometryType string            `json:"geometryType"`
	BBox         *BoundingBoxDTO   `json:"bbox,omitempty"`
	Properties   map[string]string `json:"properties"`
}

// FileInfo represents information about an opened file
type FileInfo struct {
	Path           string `json:"path"`
	DatasetCount   int    `json:"datasetCount"`
	FileGeneration uint64 `json:"fileGeneration"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// OpenFileDialog opens a native file dialog for selecting .udbx files
func (a *App) OpenFileDialog() (string, error) {
	selection, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "打开 UDBX 文件",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "UDBX Files (*.udbx)",
				Pattern:     "*.udbx",
			},
			{
				DisplayName: "All Files (*.*)",
				Pattern:     "*.*",
			},
		},
	})
	return selection, err
}

// OpenUDBXFile opens a UDBX file and returns file information
func (a *App) OpenUDBXFile(path string) (*FileInfo, error) {
	a.dataSourceLifecycleMu.Lock()
	defer a.dataSourceLifecycleMu.Unlock()

	if err := a.closeCurrentDataSource(); err != nil {
		return nil, fmt.Errorf("无法关闭当前文件: %w", err)
	}

	ds, err := udbx4go.Open(path)
	if err != nil {
		return nil, fmt.Errorf("无法打开文件: %w", err)
	}

	datasets, err := ds.ListDatasets()
	if err != nil {
		ds.Close()
		return nil, fmt.Errorf("无法读取数据集列表: %w", err)
	}

	a.dataSourceMu.Lock()
	dataSourceContext, dataSourceCancel := context.WithCancel(a.applicationContext())
	a.dataSource = ds
	a.dataSourceQueries = &sync.WaitGroup{}
	a.dataSourceContext = dataSourceContext
	a.dataSourceCancel = dataSourceCancel
	a.currentPath = path
	a.currentDatasetCount = len(datasets)
	a.fileGeneration++
	fileGeneration := a.fileGeneration
	a.dataSourceMu.Unlock()

	return &FileInfo{
		Path:           path,
		DatasetCount:   len(datasets),
		FileGeneration: fileGeneration,
	}, nil
}

// CloseUDBXFile closes the current UDBX file
func (a *App) CloseUDBXFile() error {
	a.dataSourceLifecycleMu.Lock()
	defer a.dataSourceLifecycleMu.Unlock()
	return a.closeCurrentDataSource()
}

func (a *App) closeCurrentDataSource() error {
	a.dataSourceMu.Lock()
	dataSource := a.dataSource
	queries := a.dataSourceQueries
	cancel := a.dataSourceCancel
	if dataSource != nil {
		a.fileGeneration++
	}
	a.dataSource = nil
	a.dataSourceQueries = nil
	a.dataSourceContext = nil
	a.dataSourceCancel = nil
	a.currentPath = ""
	a.currentDatasetCount = 0
	if cancel != nil {
		cancel()
	}
	a.dataSourceMu.Unlock()

	if dataSource == nil {
		return nil
	}
	if queries != nil {
		queries.Wait()
	}
	return dataSource.Close()
}

func (a *App) acquireDataSource() (*udbx4go.DataSource, uint64, context.Context, func(), error) {
	a.dataSourceMu.Lock()
	defer a.dataSourceMu.Unlock()
	if a.dataSource == nil {
		return nil, 0, nil, nil, fmt.Errorf("没有打开的文件")
	}
	if a.dataSourceQueries == nil {
		a.dataSourceQueries = &sync.WaitGroup{}
	}
	queries := a.dataSourceQueries
	queries.Add(1)
	return a.dataSource, a.fileGeneration, a.dataSourceContext, queries.Done, nil
}

// ListDatasets returns a list of all datasets in the current file
func (a *App) ListDatasets() ([]DatasetInfoDTO, error) {
	dataSource, _, _, release, err := a.acquireDataSource()
	if err != nil {
		return nil, err
	}
	defer release()

	datasets, err := dataSource.ListDatasets()
	if err != nil {
		return nil, err
	}

	result := make([]DatasetInfoDTO, len(datasets))
	for i, ds := range datasets {
		result[i] = DatasetInfoDTO{
			Name:        ds.Name,
			Kind:        ds.Kind.String(),
			ObjectCount: ds.ObjectCount,
			IconType:    getIconType(ds.Kind),
		}
	}

	return result, nil
}

// GetDatasetFields returns the fields for a specific dataset
func (a *App) GetDatasetFields(datasetName string) ([]string, error) {
	dataSource, _, _, release, err := a.acquireDataSource()
	if err != nil {
		return nil, err
	}
	defer release()

	ds, err := dataSource.GetDataset(datasetName)
	if err != nil {
		return nil, err
	}

	fields, err := ds.GetFields()
	if err != nil {
		return nil, err
	}

	result := make([]string, len(fields))
	for i, f := range fields {
		result[i] = f.Name
	}

	return result, nil
}

// LoadDatasetPage loads a page of data from a dataset
func (a *App) LoadDatasetPage(datasetName string, page int) (*PageData, error) {
	dataSource, _, _, release, err := a.acquireDataSource()
	if err != nil {
		return nil, err
	}
	defer release()

	// Get dataset info from ListDatasets
	datasets, err := dataSource.ListDatasets()
	if err != nil {
		return nil, err
	}

	var info *types.DatasetInfo
	for _, ds := range datasets {
		if ds.Name == datasetName {
			info = ds
			break
		}
	}
	if info == nil {
		return nil, fmt.Errorf("数据集不存在: %s", datasetName)
	}

	ds, err := dataSource.GetDataset(datasetName)
	if err != nil {
		return nil, err
	}

	fields, err := ds.GetFields()
	if err != nil {
		return nil, err
	}

	// Build column headers
	columns := []string{"SmID"}
	if info.Kind != types.DatasetKindTabular {
		columns = append(columns, "Geometry")
	}
	for _, f := range fields {
		columns = append(columns, f.Name)
	}

	// Calculate pagination
	totalPages := (info.ObjectCount + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * pageSize
	opts := &types.QueryOptions{
		Limit:  pageSize,
		Offset: offset,
	}

	// Load data based on dataset type using List() method via type assertion
	var rows [][]string

	// Try to get the dataset with List method
	if vectorDs, ok := ds.(interface {
		List(opts *types.QueryOptions) ([]*types.Feature, error)
	}); ok {
		features, err := vectorDs.List(opts)
		if err != nil {
			return nil, err
		}
		rows = a.formatFeatures(features, fields, info.Kind)
	} else if tabularDs, ok := ds.(interface {
		List(opts *types.QueryOptions) ([]*types.TabularRecord, error)
	}); ok {
		records, err := tabularDs.List(opts)
		if err != nil {
			return nil, err
		}
		rows = a.formatTabularRecords(records, fields)
	}

	return &PageData{
		Rows:        rows,
		Columns:     columns,
		CurrentPage: page,
		TotalPages:  totalPages,
	}, nil
}

// GetDatasetSpatialSummary returns a bounded summary for spatial preview.
func (a *App) GetDatasetSpatialSummary(datasetName string) (*SpatialSummaryDTO, error) {
	dataSource, _, dataSourceContext, release, err := a.acquireDataSource()
	if err != nil {
		return nil, err
	}
	defer release()

	info, _, err := a.getDatasetForPreview(dataSource, datasetName)
	if err != nil {
		return nil, err
	}

	summary := &SpatialSummaryDTO{
		DatasetName:      info.Name,
		Kind:             info.Kind.String(),
		SRID:             info.SRID,
		Extent:           boundingBoxDTO(info.Extent),
		ObjectCount:      info.ObjectCount,
		PreviewSupported: info.Kind.IsSpatial(),
	}
	if !summary.PreviewSupported {
		summary.UnsupportedReason = "非空间数据集不支持空间预览"
		summary.QueryDiagnosticReason = string(types.SpatialQueryReasonUnsupportedDatasetKind)
		return summary, nil
	}
	if !supportsViewportSpatialQuery(info.Kind) {
		summary.QueryDiagnosticReason = string(types.SpatialQueryReasonUnsupportedDatasetKind)
		return summary, nil
	}

	queryContext, cancel := a.spatialQueryContext(dataSourceContext)
	defer cancel()
	capability, err := dataSource.GetSpatialQueryCapability(queryContext, datasetName)
	if err != nil {
		return nil, err
	}
	summary.ViewportQuerySupported = capability.Supported
	summary.RTreeAvailable = capability.RTreeAvailable
	summary.QueryDiagnosticReason = string(capability.DiagnosticReason)
	return summary, nil
}

// LoadSpatialPreview returns a bounded renderer-neutral geometry payload.
func (a *App) LoadSpatialPreview(datasetName string, request SpatialPreviewRequestDTO) (*SpatialPreviewDTO, error) {
	dataSource, generation, dataSourceContext, release, err := a.acquireDataSource()
	if err != nil {
		return nil, err
	}
	defer release()

	info, ds, err := a.getDatasetForPreview(dataSource, datasetName)
	if err != nil {
		return nil, err
	}
	if !info.Kind.IsSpatial() {
		return nil, fmt.Errorf("数据集 %s 不支持空间预览: 非空间数据集", datasetName)
	}
	request.RequiredIDs, err = normalizeViewerRequiredIDs(request.RequiredIDs)
	if err != nil {
		return nil, err
	}
	settings, err := a.GetViewerSettings()
	if err != nil {
		defaults := DefaultViewerSettings()
		settings = &defaults
	}
	if request.Limit <= 0 {
		request.Limit = settings.SpatialPreview.FeatureLimit
	}
	if request.MaxVertices <= 0 {
		request.MaxVertices = settings.SpatialPreview.VertexBudget
	}
	request.Limit = clampInt(request.Limit, minSpatialPreviewFeatureLimit, maxSpatialPreviewFeatureLimit)
	request.MaxVertices = clampInt(request.MaxVertices, minSpatialPreviewVertexBudget, maxSpatialPreviewVertexBudget)

	fields, err := ds.GetFields()
	if err != nil {
		return nil, err
	}
	queryContext, cancel := a.spatialQueryContext(dataSourceContext)
	defer cancel()
	if a.previewQueryHook != nil {
		if err := a.previewQueryHook(queryContext); err != nil {
			return nil, mapViewerPreviewError(queryContext, err)
		}
	}

	var (
		features       []*types.Feature
		queriedBounds  *BoundingBoxDTO
		strategy       = spatialPreviewStrategyBoundedSample
		hasMore        bool
		degradedReason string
	)
	queryStarted := time.Now()
	if supportsViewportSpatialQuery(info.Kind) && request.Viewport != nil {
		queryResult, queryErr := dataSource.QuerySpatial(queryContext, datasetName, types.SpatialQueryOptions{
			Bounds:      request.Viewport.spatialBoundingBox(),
			Limit:       request.Limit,
			RequiredIDs: request.RequiredIDs,
		})
		if queryErr != nil {
			reason, ok := udbxerrors.SpatialQueryReasonOf(queryErr)
			if !ok || reason != types.SpatialQueryReasonEnvelopeCacheBudgetExceeded {
				return nil, queryErr
			}
			features, hasMore, err = a.loadBoundedPreviewFeatures(
				queryContext,
				ds,
				request.Limit,
				request.RequiredIDs,
			)
			if err != nil {
				return nil, err
			}
			requestedBounds := request.Viewport.spatialBoundingBox()
			queriedBounds = boundingBoxDTO(&requestedBounds)
			degradedReason = string(reason)
		} else {
			features = queryResult.Features
			queriedBounds = boundingBoxDTO(&queryResult.QueriedBounds)
			strategy = string(queryResult.Strategy)
			hasMore = queryResult.HasMore
		}
	} else {
		features, hasMore, err = a.loadBoundedPreviewFeatures(
			queryContext,
			ds,
			request.Limit,
			request.RequiredIDs,
		)
		if err != nil {
			return nil, err
		}
		if !supportsViewportSpatialQuery(info.Kind) {
			degradedReason = string(types.SpatialQueryReasonUnsupportedDatasetKind)
		}
	}
	queryDurationMS := float64(time.Since(queryStarted).Nanoseconds()) / float64(time.Millisecond)
	previewFeatures, vertexBudgetReached := a.formatPreviewFeatures(
		features,
		fields,
		request.MaxVertices,
		requiredIDSet(request.RequiredIDs),
	)

	response := &SpatialPreviewDTO{
		DatasetName:     info.Name,
		Kind:            info.Kind.String(),
		SRID:            info.SRID,
		Features:        previewFeatures,
		QueriedBounds:   queriedBounds,
		Strategy:        strategy,
		HasMore:         hasMore,
		DegradedReason:  degradedReason,
		QueryDurationMS: queryDurationMS,
		FileGeneration:  generation,
	}
	for _, feature := range previewFeatures {
		response.EstimatedVertexCount += countPreviewVertices(feature.Geometry)
		response.Extent = mergeBBox(response.Extent, feature.BBox)
	}
	if request.Viewport != nil && supportsViewportSpatialQuery(info.Kind) {
		applySpatialPreviewSampling(response, vertexBudgetReached)
	} else {
		response.Sampled, response.SampleReason = spatialPreviewSampleReason(
			info.ObjectCount,
			len(features),
			request.Limit,
			vertexBudgetReached,
		)
	}
	if a.previewResultHook != nil {
		a.previewResultHook(queryContext)
	}
	if err := a.validatePreviewLifecycle(queryContext, dataSource, generation); err != nil {
		return nil, err
	}
	return response, nil
}

func (a *App) validatePreviewLifecycle(
	ctx context.Context,
	dataSource *udbx4go.DataSource,
	generation uint64,
) error {
	if err := ctx.Err(); err != nil {
		return newViewerSpatialError(types.SpatialQueryReasonQueryTimeout, err)
	}
	a.dataSourceMu.Lock()
	current := a.dataSource == dataSource && a.fileGeneration == generation
	a.dataSourceMu.Unlock()
	if !current {
		return newViewerSpatialError(types.SpatialQueryReasonQueryTimeout, context.Canceled)
	}
	return nil
}

func applySpatialPreviewSampling(response *SpatialPreviewDTO, vertexBudgetReached bool) {
	response.Sampled, response.SampleReason = spatialPreviewResultSampleReason(
		response.HasMore,
		vertexBudgetReached,
	)
}

// GetFeatureAttributes returns attributes for one feature by SmID.
func (a *App) GetFeatureAttributes(datasetName string, featureID int) (*FeatureAttributesDTO, error) {
	dataSource, _, dataSourceContext, release, err := a.acquireDataSource()
	if err != nil {
		return nil, err
	}
	defer release()

	info, ds, err := a.getDatasetForPreview(dataSource, datasetName)
	if err != nil {
		return nil, err
	}
	if !info.Kind.IsSpatial() {
		return nil, fmt.Errorf("数据集 %s 不支持空间要素查询: 非空间数据集", datasetName)
	}

	fields, err := ds.GetFields()
	if err != nil {
		return nil, err
	}
	features, err := a.listPreviewFeatures(dataSourceContext, ds, &types.QueryOptions{IDs: []int{featureID}, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(features) == 0 {
		return nil, fmt.Errorf("要素不存在: %d", featureID)
	}

	feature := features[0]
	properties := make(map[string]string)
	for _, field := range fields {
		if value, ok := feature.Attributes[field.Name]; ok && value != nil {
			properties[field.Name] = fmt.Sprintf("%v", value)
		}
	}

	geometryType := ""
	var bbox *BoundingBoxDTO
	if feature.Geometry != nil {
		geometryType = feature.Geometry.GeometryType()
		bbox = bboxFromSlice(feature.Geometry.GetBBox())
	}

	return &FeatureAttributesDTO{
		DatasetName:  info.Name,
		ID:           feature.ID,
		GeometryType: geometryType,
		BBox:         bbox,
		Properties:   properties,
	}, nil
}

func (a *App) getDatasetForPreview(dataSource *udbx4go.DataSource, datasetName string) (*types.DatasetInfo, interface {
	GetFields() ([]*types.FieldInfo, error)
}, error) {
	datasets, err := dataSource.ListDatasets()
	if err != nil {
		return nil, nil, err
	}
	var info *types.DatasetInfo
	for _, ds := range datasets {
		if ds.Name == datasetName {
			info = ds
			break
		}
	}
	if info == nil {
		return nil, nil, fmt.Errorf("数据集不存在: %s", datasetName)
	}

	ds, err := dataSource.GetDataset(datasetName)
	if err != nil {
		return nil, nil, err
	}
	fielded, ok := ds.(interface {
		GetFields() ([]*types.FieldInfo, error)
	})
	if !ok {
		return nil, nil, fmt.Errorf("数据集不支持字段读取: %s", datasetName)
	}
	return info, fielded, nil
}

func (a *App) listPreviewFeatures(ctx context.Context, ds interface{}, opts *types.QueryOptions) ([]*types.Feature, error) {
	if vectorDs, ok := ds.(interface {
		ListContext(ctx context.Context, opts *types.QueryOptions) ([]*types.Feature, error)
	}); ok {
		return vectorDs.ListContext(ctx, opts)
	}
	return nil, fmt.Errorf("数据集不支持空间要素读取")
}

func (a *App) loadBoundedPreviewFeatures(
	ctx context.Context,
	ds interface{},
	limit int,
	requiredIDs []int,
) ([]*types.Feature, bool, error) {
	features, err := a.listPreviewFeatures(ctx, ds, &types.QueryOptions{Limit: limit + 1})
	if err != nil {
		return nil, false, err
	}
	hasMore := len(features) > limit
	if hasMore {
		features = features[:limit]
	}
	if len(requiredIDs) == 0 {
		return features, hasMore, nil
	}

	required, err := a.listPreviewFeatures(ctx, ds, &types.QueryOptions{
		IDs:   requiredIDs,
		Limit: len(requiredIDs),
	})
	if err != nil {
		return nil, false, err
	}
	return append(features, required...), hasMore, nil
}

func (a *App) formatPreviewFeatures(
	features []*types.Feature,
	fields []*types.FieldInfo,
	maxVertices int,
	requiredIDs map[int]struct{},
) ([]PreviewFeatureDTO, bool) {
	var ordinary []PreviewFeatureDTO
	var required []PreviewFeatureDTO
	seenRequired := make(map[int]struct{}, len(requiredIDs))
	vertexCount := 0
	vertexBudgetReached := false
	for _, feature := range features {
		geometry := toPreviewGeometry(feature.Geometry)
		if geometry.Type == "" {
			continue
		}
		props := make(map[string]string)
		for _, field := range fields {
			if value, ok := feature.Attributes[field.Name]; ok && value != nil {
				props[field.Name] = fmt.Sprintf("%v", value)
			}
		}
		formatted := PreviewFeatureDTO{
			ID:         feature.ID,
			Geometry:   geometry,
			BBox:       bboxFromSlice(feature.Geometry.GetBBox()),
			Properties: props,
		}
		if _, isRequired := requiredIDs[feature.ID]; isRequired {
			if _, exists := seenRequired[feature.ID]; !exists {
				required = append(required, formatted)
				seenRequired[feature.ID] = struct{}{}
			}
			continue
		}
		if vertexBudgetReached {
			continue
		}
		featureVertices := countPreviewVertices(geometry)
		if vertexCount+featureVertices > maxVertices {
			vertexBudgetReached = true
			continue
		}
		vertexCount += featureVertices
		ordinary = append(ordinary, formatted)
	}
	return append(ordinary, required...), vertexBudgetReached
}

func requiredIDSet(ids []int) map[int]struct{} {
	if len(ids) == 0 {
		return nil
	}
	result := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id > 0 {
			result[id] = struct{}{}
		}
	}
	return result
}

func normalizeViewerRequiredIDs(ids []int) ([]int, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	normalized := make([]int, 0, 1)
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, invalidViewerSpatialRequest("required feature ID must be positive")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
		if len(normalized) > 1 {
			return nil, invalidViewerSpatialRequest("viewer spatial preview supports one required feature")
		}
	}
	return normalized, nil
}

func invalidViewerSpatialRequest(message string) error {
	return newViewerSpatialError(
		types.SpatialQueryReasonInvalidViewport,
		udbxerrors.ConstraintError(message),
	)
}

func mapViewerPreviewError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return newViewerSpatialError(types.SpatialQueryReasonQueryTimeout, ctxErr)
	}
	return err
}

func newViewerSpatialError(reason types.SpatialQueryReason, cause error) error {
	spatialErr, err := udbx4go.NewSpatialQueryError(reason, cause)
	if err != nil {
		return err
	}
	return spatialErr
}

func spatialPreviewSampleReason(objectCount int, rawFeatureCount int, featureLimit int, vertexBudgetReached bool) (bool, string) {
	sampleReasons := make([]string, 0, 2)
	if objectCount > rawFeatureCount && rawFeatureCount >= featureLimit {
		sampleReasons = append(sampleReasons, "预览达到要素上限")
	}
	if vertexBudgetReached {
		sampleReasons = append(sampleReasons, "预览达到顶点上限")
	}
	if len(sampleReasons) == 0 {
		return false, ""
	}
	return true, strings.Join(sampleReasons, "，")
}

func spatialPreviewResultSampleReason(hasMore bool, vertexBudgetReached bool) (bool, string) {
	sampleReasons := make([]string, 0, 2)
	if hasMore {
		sampleReasons = append(sampleReasons, "预览达到要素上限")
	}
	if vertexBudgetReached {
		sampleReasons = append(sampleReasons, "预览达到顶点上限")
	}
	if len(sampleReasons) == 0 {
		return false, ""
	}
	return true, strings.Join(sampleReasons, "，")
}

func supportsViewportSpatialQuery(kind types.DatasetKind) bool {
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

func (b *BoundingBoxDTO) spatialBoundingBox() types.BoundingBox {
	if b == nil {
		return types.BoundingBox{}
	}
	return types.BoundingBox{MinX: b.MinX, MinY: b.MinY, MaxX: b.MaxX, MaxY: b.MaxY}
}

func boundingBoxDTO(bounds *types.BoundingBox) *BoundingBoxDTO {
	if bounds == nil {
		return nil
	}
	return &BoundingBoxDTO{MinX: bounds.MinX, MinY: bounds.MinY, MaxX: bounds.MaxX, MaxY: bounds.MaxY}
}

func (a *App) applicationContext() context.Context {
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	return base
}

func (a *App) spatialQueryContext(base context.Context) (context.Context, context.CancelFunc) {
	if base == nil {
		base = a.applicationContext()
	}
	return context.WithTimeout(base, viewerSpatialQueryTimeout)
}

// formatFeatures formats feature data for display
func (a *App) formatFeatures(features []*types.Feature, fields []*types.FieldInfo, kind types.DatasetKind) [][]string {
	var rows [][]string
	for _, f := range features {
		row := []string{strconv.Itoa(f.ID)}

		if kind != types.DatasetKindTabular {
			geom := formatGeometry(f.Geometry)
			row = append(row, geom)
		}

		for _, field := range fields {
			val := ""
			if v, ok := f.Attributes[field.Name]; ok && v != nil {
				val = fmt.Sprintf("%v", v)
			}
			row = append(row, val)
		}
		rows = append(rows, row)
	}
	return rows
}

// formatTabularRecords formats tabular records for display
func (a *App) formatTabularRecords(records []*types.TabularRecord, fields []*types.FieldInfo) [][]string {
	var rows [][]string
	for _, r := range records {
		row := []string{strconv.Itoa(r.ID)}
		for _, field := range fields {
			val := ""
			if v, ok := r.Attributes[field.Name]; ok && v != nil {
				val = fmt.Sprintf("%v", v)
			}
			row = append(row, val)
		}
		rows = append(rows, row)
	}
	return rows
}

func toPreviewGeometry(g types.Geometry) PreviewGeometryDTO {
	if g == nil {
		return PreviewGeometryDTO{}
	}
	switch geom := g.(type) {
	case *types.PointGeometry:
		return PreviewGeometryDTO{Type: "Point", Coordinates: floatSliceToInterfaces(geom.Coordinates), HasZ: geom.HasZ()}
	case *types.MultiLineStringGeometry:
		return PreviewGeometryDTO{Type: "MultiLineString", Coordinates: multiLineToInterfaces(geom.Coordinates), HasZ: geom.HasZ()}
	case *types.MultiPolygonGeometry:
		return PreviewGeometryDTO{Type: "MultiPolygon", Coordinates: multiPolygonToInterfaces(geom.Coordinates), HasZ: geom.HasZ()}
	case *types.TextGeometry:
		return PreviewGeometryDTO{Type: "Text", Coordinates: floatSliceToInterfaces(geom.Anchor), HasZ: geom.HasZ()}
	case *types.CadPointGeometry:
		return PreviewGeometryDTO{Type: "Point", Coordinates: floatSliceToInterfaces([]float64{geom.XCoord, geom.YCoord}), HasZ: false}
	case *types.CadLineGeometry:
		return PreviewGeometryDTO{Type: "MultiLineString", Coordinates: cadLineToInterfaces(geom.Coordinates, geom.SubPointCounts), HasZ: false}
	case *types.CadRegionGeometry:
		return PreviewGeometryDTO{Type: "MultiPolygon", Coordinates: cadRegionToInterfaces(geom.Coordinates, geom.SubPointCounts), HasZ: false}
	case *types.CadTextGeometry:
		return PreviewGeometryDTO{Type: "Text", Coordinates: floatSliceToInterfaces(geom.Anchor), HasZ: false}
	default:
		return PreviewGeometryDTO{Type: g.GeometryType(), Coordinates: []interface{}{}, HasZ: g.HasZ()}
	}
}

func floatSliceToInterfaces(values []float64) []interface{} {
	result := make([]interface{}, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}

func multiLineToInterfaces(lines [][][]float64) []interface{} {
	result := make([]interface{}, 0, len(lines))
	for _, line := range lines {
		points := make([]interface{}, 0, len(line))
		for _, point := range line {
			points = append(points, floatSliceToInterfaces(point))
		}
		result = append(result, points)
	}
	return result
}

func multiPolygonToInterfaces(polygons [][][][]float64) []interface{} {
	result := make([]interface{}, 0, len(polygons))
	for _, polygon := range polygons {
		rings := make([]interface{}, 0, len(polygon))
		for _, ring := range polygon {
			points := make([]interface{}, 0, len(ring))
			for _, point := range ring {
				points = append(points, floatSliceToInterfaces(point))
			}
			rings = append(rings, points)
		}
		result = append(result, rings)
	}
	return result
}

func cadLineToInterfaces(coordinates [][2]float64, subPointCounts []int) []interface{} {
	lines := splitCADCoordinates(coordinates, subPointCounts)
	result := make([]interface{}, 0, len(lines))
	for _, line := range lines {
		points := make([]interface{}, 0, len(line))
		for _, point := range line {
			points = append(points, []interface{}{point[0], point[1]})
		}
		result = append(result, points)
	}
	return result
}

func cadRegionToInterfaces(coordinates [][2]float64, subPointCounts []int) []interface{} {
	rings := cadLineToInterfaces(coordinates, subPointCounts)
	return []interface{}{rings}
}

func splitCADCoordinates(coordinates [][2]float64, subPointCounts []int) [][][2]float64 {
	if len(subPointCounts) == 0 {
		return [][][2]float64{coordinates}
	}
	var result [][][2]float64
	offset := 0
	for _, count := range subPointCounts {
		if count <= 0 || offset >= len(coordinates) {
			continue
		}
		end := offset + count
		if end > len(coordinates) {
			end = len(coordinates)
		}
		result = append(result, coordinates[offset:end])
		offset = end
	}
	return result
}

func bboxFromSlice(values []float64) *BoundingBoxDTO {
	if len(values) < 4 {
		return nil
	}
	return &BoundingBoxDTO{MinX: values[0], MinY: values[1], MaxX: values[2], MaxY: values[3]}
}

func mergeBBox(current *BoundingBoxDTO, next *BoundingBoxDTO) *BoundingBoxDTO {
	if next == nil {
		return current
	}
	if current == nil {
		return &BoundingBoxDTO{MinX: next.MinX, MinY: next.MinY, MaxX: next.MaxX, MaxY: next.MaxY}
	}
	if next.MinX < current.MinX {
		current.MinX = next.MinX
	}
	if next.MinY < current.MinY {
		current.MinY = next.MinY
	}
	if next.MaxX > current.MaxX {
		current.MaxX = next.MaxX
	}
	if next.MaxY > current.MaxY {
		current.MaxY = next.MaxY
	}
	return current
}

func countPreviewVertices(geometry PreviewGeometryDTO) int {
	return countCoordinateVertices(geometry.Coordinates)
}

func countCoordinateVertices(values []interface{}) int {
	if len(values) == 0 {
		return 0
	}
	if _, ok := values[0].(float64); ok {
		return 1
	}
	total := 0
	for _, value := range values {
		if nested, ok := value.([]interface{}); ok {
			total += countCoordinateVertices(nested)
		}
	}
	return total
}

// formatGeometry formats a geometry for display
func formatGeometry(g types.Geometry) string {
	if g == nil {
		return "(null)"
	}

	switch geom := g.(type) {
	case *types.PointGeometry:
		if geom.HasZ() {
			return fmt.Sprintf("POINT Z(%.2f %.2f %.2f)", geom.X(), geom.Y(), geom.Z())
		}
		return fmt.Sprintf("POINT(%.2f %.2f)", geom.X(), geom.Y())
	case *types.MultiLineStringGeometry:
		return fmt.Sprintf("Line[%d]", len(geom.Coordinates))
	case *types.MultiPolygonGeometry:
		return fmt.Sprintf("Region[%d]", len(geom.Coordinates))
	default:
		return g.GeometryType()
	}
}

// getIconType returns the icon type for a dataset kind
func getIconType(kind types.DatasetKind) string {
	switch kind {
	case types.DatasetKindPoint, types.DatasetKindPointZ:
		return "point"
	case types.DatasetKindLine, types.DatasetKindLineZ:
		return "line"
	case types.DatasetKindRegion, types.DatasetKindRegionZ:
		return "region"
	case types.DatasetKindText:
		return "text"
	case types.DatasetKindCAD:
		return "cad"
	case types.DatasetKindTabular:
		return "tabular"
	default:
		return "unknown"
	}
}

// GetCurrentFile returns the current file path
func (a *App) GetCurrentFile() string {
	a.dataSourceMu.Lock()
	defer a.dataSourceMu.Unlock()
	return a.currentPath
}

// GetCurrentFileInfo returns the backend-authoritative current file state.
func (a *App) GetCurrentFileInfo() *FileInfo {
	a.dataSourceMu.Lock()
	defer a.dataSourceMu.Unlock()
	if a.dataSource == nil {
		return nil
	}
	return &FileInfo{
		Path:           a.currentPath,
		DatasetCount:   a.currentDatasetCount,
		FileGeneration: a.fileGeneration,
	}
}
