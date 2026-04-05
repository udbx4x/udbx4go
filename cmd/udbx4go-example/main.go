package main

import (
	"fmt"
	"log"
	"os"

	"github.com/udbx4x/udbx4go"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func main() {
	// Create a new UDBX file
	udbxPath := "example.udbx"
	defer os.Remove(udbxPath) // Clean up after example

	fmt.Println("=== Creating UDBX file ===")
	ds, err := udbx4go.Create(udbxPath)
	if err != nil {
		log.Fatal("Failed to create UDBX file:", err)
	}

	// Create a point dataset for cities
	fmt.Println("\n=== Creating Point Dataset ===")
	fields := []*types.FieldInfo{
		{Name: "name", FieldType: udbx4go.FieldTypeText, Nullable: false},
		{Name: "population", FieldType: udbx4go.FieldTypeInt32, Nullable: true},
	}

	pointDS, err := ds.CreatePointDataset("cities", 4326, fields)
	if err != nil {
		log.Fatal("Failed to create point dataset:", err)
	}
	fmt.Printf("Created dataset: %s (kind: %s, SRID: %d)\n",
		pointDS.Info().Name, pointDS.Info().Kind.String(), pointDS.SRID())

	// Insert some features
	fmt.Println("\n=== Inserting Features ===")
	features := []*udbx4go.Feature{
		{
			ID: 1,
			Geometry: udbx4go.PointGeometry{
				Type:        "Point",
				Coordinates: []float64{116.4, 39.9}, // Beijing
			},
			Attributes: map[string]interface{}{
				"name":       "Beijing",
				"population": 21540000,
			},
		},
		{
			ID: 2,
			Geometry: udbx4go.PointGeometry{
				Type:        "Point",
				Coordinates: []float64{121.5, 31.2}, // Shanghai
			},
			Attributes: map[string]interface{}{
				"name":       "Shanghai",
				"population": 24280000,
			},
		},
		{
			ID: 3,
			Geometry: udbx4go.PointGeometry{
				Type:        "Point",
				Coordinates: []float64{113.3, 23.1}, // Guangzhou
			},
			Attributes: map[string]interface{}{
				"name":       "Guangzhou",
				"population": 14043500,
			},
		},
	}

	for _, f := range features {
		if err := pointDS.Insert(f); err != nil {
			log.Printf("Failed to insert feature %d: %v", f.ID, err)
		} else {
			fmt.Printf("Inserted: %s (ID: %d)\n", f.Attributes["name"], f.ID)
		}
	}

	// List all features
	fmt.Println("\n=== Listing Features ===")
	allFeatures, err := pointDS.List(nil)
	if err != nil {
		log.Fatal("Failed to list features:", err)
	}

	fmt.Printf("Total features: %d\n", len(allFeatures))
	for _, f := range allFeatures {
		geom := f.Geometry.(*udbx4go.PointGeometry)
		fmt.Printf("  ID: %d, Name: %s, Coordinates: [%.2f, %.2f]\n",
			f.ID, f.Attributes["name"], geom.X(), geom.Y())
	}

	// Query specific feature
	fmt.Println("\n=== Querying Feature by ID ===")
	feature, err := pointDS.GetByID(2)
	if err != nil {
		log.Printf("Failed to get feature: %v", err)
	} else {
		fmt.Printf("Found feature: %s (population: %d)\n",
			feature.Attributes["name"], feature.Attributes["population"])
	}

	// Create a tabular dataset
	fmt.Println("\n=== Creating Tabular Dataset ===")
	countryFields := []*types.FieldInfo{
		{Name: "code", FieldType: udbx4go.FieldTypeText, Nullable: false},
		{Name: "name", FieldType: udbx4go.FieldTypeText, Nullable: false},
	}

	tabularDS, err := ds.CreateTabularDataset("countries", countryFields)
	if err != nil {
		log.Fatal("Failed to create tabular dataset:", err)
	}
	fmt.Printf("Created dataset: %s (kind: %s)\n",
		tabularDS.Info().Name, tabularDS.Info().Kind.String())

	// Insert tabular records
	records := []*udbx4go.TabularRecord{
		{
			ID: 1,
			Attributes: map[string]interface{}{
				"code": "CN",
				"name": "China",
			},
		},
		{
			ID: 2,
			Attributes: map[string]interface{}{
				"code": "US",
				"name": "United States",
			},
		},
	}

	for _, r := range records {
		if err := tabularDS.Insert(r); err != nil {
			log.Printf("Failed to insert record %d: %v", r.ID, err)
		}
	}

	// List all datasets
	fmt.Println("\n=== Listing All Datasets ===")
	datasets, err := ds.ListDatasets()
	if err != nil {
		log.Fatal("Failed to list datasets:", err)
	}

	for _, d := range datasets {
		fmt.Printf("  - %s (kind: %s, records: %d)\n", d.Name, d.Kind.String(), d.ObjectCount)
	}

	// Close the data source
	fmt.Println("\n=== Closing ===")
	if err := ds.Close(); err != nil {
		log.Fatal("Failed to close data source:", err)
	}
	fmt.Println("Successfully closed UDBX file")

	// Reopen and verify
	fmt.Println("\n=== Reopening and Verifying ===")
	ds2, err := udbx4go.Open(udbxPath)
	if err != nil {
		log.Fatal("Failed to reopen UDBX file:", err)
	}
	defer ds2.Close()

	datasets2, err := ds2.ListDatasets()
	if err != nil {
		log.Fatal("Failed to list datasets:", err)
	}
	fmt.Printf("Reopened file with %d datasets\n", len(datasets2))

	// Get cities dataset
	citiesDS, err := ds2.GetPointDataset("cities")
	if err != nil {
		log.Fatal("Failed to get cities dataset:", err)
	}

	cities, err := citiesDS.List(nil)
	if err != nil {
		log.Fatal("Failed to list cities:", err)
	}

	fmt.Printf("Verified: found %d cities\n", len(cities))
	for _, c := range cities {
		fmt.Printf("  - %s\n", c.Attributes["name"])
	}

	fmt.Println("\n=== Example completed successfully! ===")
}
