package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/udbx4x/udbx4go"
	"github.com/udbx4x/udbx4go/internal/dataset"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func main() {
	sourcePath := flag.String("source", "", "path to source compliance UDBX file")
	outputPath := flag.String("output", "", "path to output roundtrip UDBX file")
	flag.Parse()

	if *sourcePath == "" || *outputPath == "" {
		fmt.Fprintln(os.Stderr, "usage: udbx4go-roundtrip-fixture --source <input.udbx> --output <output.udbx>")
		os.Exit(2)
	}

	if err := generateRoundtripFixture(*sourcePath, *outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate roundtrip fixture: %v\n", err)
		os.Exit(1)
	}
}

func generateRoundtripFixture(sourcePath, outputPath string) error {
	source, err := udbx4go.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := udbx4go.Create(outputPath)
	if err != nil {
		return err
	}
	defer target.Close()

	for _, datasetName := range []string{
		"test_points",
		"test_lines",
		"test_regions",
		"test_points_z",
		"test_lines_z",
		"test_regions_z",
		"test_tabular",
		"test_cad",
		"test_text",
	} {
		if err := copyDataset(source, target, datasetName); err != nil {
			return fmt.Errorf("%s: %w", datasetName, err)
		}
	}

	return nil
}

func copyDataset(source, target *udbx4go.DataSource, datasetName string) error {
	sourceDataset, err := source.GetDataset(datasetName)
	if err != nil {
		return err
	}

	fields, err := sourceDataset.GetFields()
	if err != nil {
		return err
	}

	info := sourceDataset.Info()
	srid := 0
	if info.SRID != nil {
		srid = *info.SRID
	}

	switch typedDataset := sourceDataset.(type) {
	case *dataset.PointDataset:
		targetDataset, err := target.CreatePointDataset(info.Name, srid, cloneFields(fields))
		if err != nil {
			return err
		}
		features, err := typedDataset.List(nil)
		if err != nil {
			return err
		}
		return targetDataset.InsertMany(cloneFeatures(features))
	case *dataset.LineDataset:
		targetDataset, err := target.CreateLineDataset(info.Name, srid, cloneFields(fields))
		if err != nil {
			return err
		}
		features, err := typedDataset.List(nil)
		if err != nil {
			return err
		}
		return targetDataset.InsertMany(cloneFeatures(features))
	case *dataset.RegionDataset:
		targetDataset, err := target.CreateRegionDataset(info.Name, srid, cloneFields(fields))
		if err != nil {
			return err
		}
		features, err := typedDataset.List(nil)
		if err != nil {
			return err
		}
		return targetDataset.InsertMany(cloneFeatures(features))
	case *dataset.PointZDataset:
		targetDataset, err := target.CreatePointZDataset(info.Name, srid, cloneFields(fields))
		if err != nil {
			return err
		}
		features, err := typedDataset.List(nil)
		if err != nil {
			return err
		}
		return targetDataset.InsertMany(cloneFeatures(features))
	case *dataset.LineZDataset:
		targetDataset, err := target.CreateLineZDataset(info.Name, srid, cloneFields(fields))
		if err != nil {
			return err
		}
		features, err := typedDataset.List(nil)
		if err != nil {
			return err
		}
		return targetDataset.InsertMany(cloneFeatures(features))
	case *dataset.RegionZDataset:
		targetDataset, err := target.CreateRegionZDataset(info.Name, srid, cloneFields(fields))
		if err != nil {
			return err
		}
		features, err := typedDataset.List(nil)
		if err != nil {
			return err
		}
		return targetDataset.InsertMany(cloneFeatures(features))
	case *dataset.TabularDataset:
		targetDataset, err := target.CreateTabularDataset(info.Name, cloneFields(fields))
		if err != nil {
			return err
		}
		records, err := typedDataset.List(nil)
		if err != nil {
			return err
		}
		return targetDataset.InsertMany(cloneRecords(records))
	case *dataset.CadDataset:
		targetDataset, err := target.CreateCadDataset(info.Name, cloneFields(fields))
		if err != nil {
			return err
		}
		features, err := typedDataset.List(nil)
		if err != nil {
			return err
		}
		return targetDataset.InsertMany(cloneFeatures(features))
	case *dataset.TextDataset:
		targetDataset, err := target.CreateTextDataset(info.Name, srid, cloneFields(fields))
		if err != nil {
			return err
		}
		features, err := typedDataset.List(nil)
		if err != nil {
			return err
		}
		return targetDataset.InsertMany(cloneFeatures(features))
	default:
		return fmt.Errorf("unsupported dataset type %T", sourceDataset)
	}
}

func cloneFields(fields []*types.FieldInfo) []*types.FieldInfo {
	cloned := make([]*types.FieldInfo, len(fields))
	for i, field := range fields {
		copied := *field
		cloned[i] = &copied
	}
	return cloned
}

func cloneFeatures(features []*types.Feature) []*types.Feature {
	cloned := make([]*types.Feature, len(features))
	for i, feature := range features {
		attributes := make(map[string]interface{}, len(feature.Attributes))
		for key, value := range feature.Attributes {
			attributes[key] = value
		}
		cloned[i] = &types.Feature{
			ID:         feature.ID,
			Geometry:   cloneGeometry(feature.Geometry),
			Attributes: attributes,
		}
	}
	return cloned
}

func cloneRecords(records []*types.TabularRecord) []*types.TabularRecord {
	cloned := make([]*types.TabularRecord, len(records))
	for i, record := range records {
		attributes := make(map[string]interface{}, len(record.Attributes))
		for key, value := range record.Attributes {
			attributes[key] = value
		}
		cloned[i] = &types.TabularRecord{
			ID:         record.ID,
			Attributes: attributes,
		}
	}
	return cloned
}

func cloneGeometry(geometry types.Geometry) types.Geometry {
	switch geom := geometry.(type) {
	case *types.PointGeometry:
		coordinates := append([]float64(nil), geom.Coordinates...)
		bbox := append([]float64(nil), geom.BBox...)
		return &types.PointGeometry{
			Type:        geom.Type,
			Coordinates: coordinates,
			SRID:        geom.SRID,
			HasZValue:   geom.HasZValue,
			BBox:        bbox,
			GeoType:     geom.GeoType,
		}
	case *types.MultiLineStringGeometry:
		coordinates := make([][][]float64, len(geom.Coordinates))
		for i, line := range geom.Coordinates {
			coordinates[i] = make([][]float64, len(line))
			for j, point := range line {
				coordinates[i][j] = append([]float64(nil), point...)
			}
		}
		bbox := append([]float64(nil), geom.BBox...)
		return &types.MultiLineStringGeometry{
			Type:        geom.Type,
			Coordinates: coordinates,
			SRID:        geom.SRID,
			HasZValue:   geom.HasZValue,
			BBox:        bbox,
			GeoType:     geom.GeoType,
		}
	case *types.MultiPolygonGeometry:
		coordinates := make([][][][]float64, len(geom.Coordinates))
		for i, polygon := range geom.Coordinates {
			coordinates[i] = make([][][]float64, len(polygon))
			for j, ring := range polygon {
				coordinates[i][j] = make([][]float64, len(ring))
				for k, point := range ring {
					coordinates[i][j][k] = append([]float64(nil), point...)
				}
			}
		}
		bbox := append([]float64(nil), geom.BBox...)
		return &types.MultiPolygonGeometry{
			Type:        geom.Type,
			Coordinates: coordinates,
			SRID:        geom.SRID,
			HasZValue:   geom.HasZValue,
			BBox:        bbox,
			GeoType:     geom.GeoType,
		}
	case *types.CadPointGeometry:
		return &types.CadPointGeometry{
			XCoord: geom.XCoord,
			YCoord: geom.YCoord,
			Style:  cloneCadStyle(geom.Style),
		}
	case *types.CadLineGeometry:
		return &types.CadLineGeometry{
			NumSub:         geom.NumSub,
			SubPointCounts: append([]int(nil), geom.SubPointCounts...),
			Coordinates:    append([][2]float64(nil), geom.Coordinates...),
			Style:          cloneCadStyle(geom.Style),
		}
	case *types.CadRegionGeometry:
		return &types.CadRegionGeometry{
			NumSub:         geom.NumSub,
			SubPointCounts: append([]int(nil), geom.SubPointCounts...),
			Coordinates:    append([][2]float64(nil), geom.Coordinates...),
			Style:          cloneCadStyle(geom.Style),
		}
	case *types.TextGeometry:
		return &types.TextGeometry{
			Type:     geom.Type,
			Text:     geom.Text,
			Anchor:   append([]float64(nil), geom.Anchor...),
			Rotation: geom.Rotation,
			SRID:     geom.SRID,
			BBox:     append([]float64(nil), geom.BBox...),
			GeoType:  geom.GeoType,
			Style:    cloneTextStyle(geom.Style),
			SubTexts: cloneTextSubTexts(geom.SubTexts),
		}
	default:
		return geometry
	}
}

func cloneCadStyle(style types.CadStyle) types.CadStyle {
	switch typedStyle := style.(type) {
	case *types.CadMarkerStyle:
		copied := *typedStyle
		return &copied
	case *types.CadLineStyle:
		copied := *typedStyle
		return &copied
	case *types.CadFillStyle:
		copied := *typedStyle
		return &copied
	default:
		return style
	}
}

func cloneTextStyle(style *types.TextStyle) *types.TextStyle {
	if style == nil {
		return nil
	}
	return &types.TextStyle{
		Color:           cloneColor(style.Color),
		BackgroundColor: cloneColor(style.BackgroundColor),
		FixedSize:       style.FixedSize,
		Weight:          style.Weight,
		StyleFlag:       style.StyleFlag,
		AlignFlag:       style.AlignFlag,
		FontWidth:       style.FontWidth,
		FontHeight:      style.FontHeight,
		Anchor:          append([]float64(nil), style.Anchor...),
		FaceName:        style.FaceName,
	}
}

func cloneColor(color *types.Color) *types.Color {
	if color == nil {
		return nil
	}
	copied := *color
	return &copied
}

func cloneTextSubTexts(subTexts []*types.TextSubText) []*types.TextSubText {
	cloned := make([]*types.TextSubText, len(subTexts))
	for i, subText := range subTexts {
		cloned[i] = &types.TextSubText{
			Text:     subText.Text,
			Anchor:   append([]float64(nil), subText.Anchor...),
			Rotation: subText.Rotation,
		}
	}
	return cloned
}
