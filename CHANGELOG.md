# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial implementation of UDBX reader/writer library
- Support for all dataset types: Point, Line, Region, PointZ, LineZ, RegionZ, Tabular
- GAIA geometry codec for binary encoding/decoding
- System table DAOs (SmRegister, SmFieldInfo, geometry_columns, SmDataSourceInfo)
- Comprehensive error handling with typed errors
- GeoJSON-like geometry model
- CRUD operations for all dataset types
- Test suite with 76%+ coverage
- Documentation (README, CLAUDE.md)

### Features

- **Open/Close**: Open existing UDBX files and create new ones
- **Dataset Management**: List, get, and create datasets
- **Point Dataset**: 2D and 3D point features with full CRUD
- **Line Dataset**: MultiLineString features with full CRUD
- **Region Dataset**: MultiPolygon features with full CRUD
- **Tabular Dataset**: Non-spatial tables with full CRUD
- **Query Options**: Limit, offset, and ID-based filtering
- **Batch Operations**: InsertMany for efficient bulk inserts

### Technical

- SQLite-based storage using `github.com/mattn/go-sqlite3`
- Little-endian GAIA geometry encoding
- TDD development approach
- Table-driven tests with testify
- Type re-exports for convenient API access
