package main

import (
	"context"
	"fmt"
	"strconv"

	udbx4go "github.com/udbx4x/udbx4go"
	"github.com/udbx4x/udbx4go/pkg/types"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const pageSize = 100

// App struct
type App struct {
	ctx         context.Context
	dataSource  *udbx4go.DataSource
	currentPath string
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
	DatasetName          string          `json:"datasetName"`
	Kind                 string          `json:"kind"`
	SRID                 *int            `json:"srid,omitempty"`
	Extent               *BoundingBoxDTO `json:"extent,omitempty"`
	ObjectCount          int             `json:"objectCount"`
	EstimatedVertexCount int             `json:"estimatedVertexCount"`
	PreviewSupported     bool            `json:"previewSupported"`
	UnsupportedReason    string          `json:"unsupportedReason,omitempty"`
}

// SpatialPreviewRequestDTO limits the amount of geometry sent to the frontend.
type SpatialPreviewRequestDTO struct {
	Viewport    *BoundingBoxDTO `json:"viewport,omitempty"`
	Limit       int             `json:"limit"`
	MaxVertices int             `json:"maxVertices"`
	Simplify    bool            `json:"simplify"`
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
}

// FileInfo represents information about an opened file
type FileInfo struct {
	Path         string `json:"path"`
	DatasetCount int    `json:"datasetCount"`
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
	// Close any existing datasource
	if a.dataSource != nil {
		a.dataSource.Close()
		a.dataSource = nil
		a.currentPath = ""
	}

	ds, err := udbx4go.Open(path)
	if err != nil {
		return nil, fmt.Errorf("无法打开文件: %w", err)
	}

	a.dataSource = ds
	a.currentPath = path

	datasets, err := ds.ListDatasets()
	if err != nil {
		ds.Close()
		a.dataSource = nil
		a.currentPath = ""
		return nil, fmt.Errorf("无法读取数据集列表: %w", err)
	}

	return &FileInfo{
		Path:         path,
		DatasetCount: len(datasets),
	}, nil
}

// CloseUDBXFile closes the current UDBX file
func (a *App) CloseUDBXFile() error {
	if a.dataSource != nil {
		a.dataSource.Close()
		a.dataSource = nil
		a.currentPath = ""
	}
	return nil
}

// ListDatasets returns a list of all datasets in the current file
func (a *App) ListDatasets() ([]DatasetInfoDTO, error) {
	if a.dataSource == nil {
		return nil, fmt.Errorf("没有打开的文件")
	}

	datasets, err := a.dataSource.ListDatasets()
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
	if a.dataSource == nil {
		return nil, fmt.Errorf("没有打开的文件")
	}

	ds, err := a.dataSource.GetDataset(datasetName)
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
	if a.dataSource == nil {
		return nil, fmt.Errorf("没有打开的文件")
	}

	// Get dataset info from ListDatasets
	datasets, err := a.dataSource.ListDatasets()
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

	ds, err := a.dataSource.GetDataset(datasetName)
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
		if err == nil {
			rows = a.formatFeatures(features, fields, info.Kind)
		}
	} else if tabularDs, ok := ds.(interface {
		List(opts *types.QueryOptions) ([]*types.TabularRecord, error)
	}); ok {
		records, err := tabularDs.List(opts)
		if err == nil {
			rows = a.formatTabularRecords(records, fields)
		}
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
	info, ds, err := a.getDatasetForPreview(datasetName)
	if err != nil {
		return nil, err
	}

	summary := &SpatialSummaryDTO{
		DatasetName:      info.Name,
		Kind:             info.Kind.String(),
		SRID:             info.SRID,
		ObjectCount:      info.ObjectCount,
		PreviewSupported: info.Kind.IsSpatial(),
	}
	if !summary.PreviewSupported {
		summary.UnsupportedReason = "非空间数据集不支持空间预览"
		return summary, nil
	}

	fields, _ := ds.GetFields()
	features, err := a.listPreviewFeatures(ds, &types.QueryOptions{Limit: 1000})
	if err != nil {
		return nil, err
	}
	previewFeatures := a.formatPreviewFeatures(features, fields, 1000000)
	for _, feature := range previewFeatures {
		summary.EstimatedVertexCount += countPreviewVertices(feature.Geometry)
		summary.Extent = mergeBBox(summary.Extent, feature.BBox)
	}
	return summary, nil
}

// LoadSpatialPreview returns a bounded renderer-neutral geometry payload.
func (a *App) LoadSpatialPreview(datasetName string, request SpatialPreviewRequestDTO) (*SpatialPreviewDTO, error) {
	info, ds, err := a.getDatasetForPreview(datasetName)
	if err != nil {
		return nil, err
	}
	if !info.Kind.IsSpatial() {
		return nil, fmt.Errorf("数据集 %s 不支持空间预览: 非空间数据集", datasetName)
	}
	if request.Limit <= 0 {
		request.Limit = 100
	}
	if request.MaxVertices <= 0 {
		request.MaxVertices = 10000
	}

	fields, err := ds.GetFields()
	if err != nil {
		return nil, err
	}
	features, err := a.listPreviewFeatures(ds, &types.QueryOptions{Limit: request.Limit})
	if err != nil {
		return nil, err
	}
	previewFeatures := a.formatPreviewFeatures(features, fields, request.MaxVertices)

	response := &SpatialPreviewDTO{
		DatasetName: info.Name,
		Kind:        info.Kind.String(),
		SRID:        info.SRID,
		Features:    previewFeatures,
	}
	for _, feature := range previewFeatures {
		response.EstimatedVertexCount += countPreviewVertices(feature.Geometry)
		response.Extent = mergeBBox(response.Extent, feature.BBox)
	}
	if response.EstimatedVertexCount >= request.MaxVertices {
		response.Sampled = true
		response.SampleReason = "预览达到顶点上限"
	}
	return response, nil
}

func (a *App) getDatasetForPreview(datasetName string) (*types.DatasetInfo, interface {
	GetFields() ([]*types.FieldInfo, error)
}, error) {
	if a.dataSource == nil {
		return nil, nil, fmt.Errorf("没有打开的文件")
	}

	datasets, err := a.dataSource.ListDatasets()
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

	ds, err := a.dataSource.GetDataset(datasetName)
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

func (a *App) listPreviewFeatures(ds interface{}, opts *types.QueryOptions) ([]*types.Feature, error) {
	if vectorDs, ok := ds.(interface {
		List(opts *types.QueryOptions) ([]*types.Feature, error)
	}); ok {
		return vectorDs.List(opts)
	}
	return nil, fmt.Errorf("数据集不支持空间要素读取")
}

func (a *App) formatPreviewFeatures(features []*types.Feature, fields []*types.FieldInfo, maxVertices int) []PreviewFeatureDTO {
	var result []PreviewFeatureDTO
	vertexCount := 0
	for _, feature := range features {
		geometry := toPreviewGeometry(feature.Geometry)
		if geometry.Type == "" {
			continue
		}
		vertexCount += countPreviewVertices(geometry)
		if vertexCount > maxVertices {
			break
		}
		props := make(map[string]string)
		for _, field := range fields {
			if value, ok := feature.Attributes[field.Name]; ok && value != nil {
				props[field.Name] = fmt.Sprintf("%v", value)
			}
		}
		result = append(result, PreviewFeatureDTO{
			ID:         feature.ID,
			Geometry:   geometry,
			BBox:       bboxFromSlice(feature.Geometry.GetBBox()),
			Properties: props,
		})
	}
	return result
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
	case types.DatasetKindTabular:
		return "tabular"
	default:
		return "unknown"
	}
}

// GetCurrentFile returns the current file path
func (a *App) GetCurrentFile() string {
	return a.currentPath
}
