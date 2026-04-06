package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	udbx4go "github.com/udbx4x/udbx4go"
	"github.com/udbx4x/udbx4go/pkg/types"
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
	if vectorDs, ok := ds.(interface{ List(opts *types.QueryOptions) ([]*types.Feature, error) }); ok {
		features, err := vectorDs.List(opts)
		if err == nil {
			rows = a.formatFeatures(features, fields, info.Kind)
		}
	} else if tabularDs, ok := ds.(interface{ List(opts *types.QueryOptions) ([]*types.TabularRecord, error) }); ok {
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
