# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Added `DataSource.QuerySpatial` and `GetSpatialQueryCapability` for Point, Line, Region, and corresponding Z datasets.
- Added closed-interval MBR queries with stable `hasMore`, required-ID, strategy, and six-reason-code semantics from `udbx4spec`.
- Added verified RTree querying and DataSource-lifetime envelope caches; cache admission failures return the stable `envelope_cache_budget_exceeded` error.
- Added context-aware `ListContext` methods for spatial, Text, and CAD datasets.
- Added Viewer-private non-spatial `bounded_sample` previews for cache-budget failures and Text/CAD paths while retaining `degradedReason` in the Viewer DTO.
- Added Viewer viewport coordination, offscreen selection retention, deterministic stale-result probes, canvas-pixel blank-render gates, real-sample automatic tests, the envelope-cache PoC, and transactional macOS benchmark tooling.

### Changed

- The current envelope-cache defaults are 32 MiB per dataset and 64 MiB per open DataSource. They are measured SDK resource policies, not UDBX format limits.
- Tightened `QuerySpatial` to the `udbx4spec` contract: successful strategies are only `rtree` and `envelope_cache`; ordinary features must intersect the requested MBR, only required IDs may be offscreen, and `SpatialQueryResult` no longer carries degraded-result diagnostics.
- Viewer query concurrency remains 1 until packaged runtime measurements for candidates 2 and 3 pass the acceptance gates.

### Known limitations

- Text and CAD viewport queries, coordinate projection, and exact topology predicates are not implemented; Text and CAD keep bounded preview/list behavior.
- SDK, Viewer, frontend, specification, and non-GUI benchmark workflow gates are automated. The previous packaged benchmark remains historical evidence because it did not deterministically create a stale request or inspect rendered canvas pixels; the tightened gates require a new packaged run.

## [0.1.0] - 2026-06-26

### Added

- Initial public release of the UDBX reader/writer library for Go.
- Support for implemented dataset types: Point, Line, Region, PointZ, LineZ, RegionZ, Tabular, Text, CAD.
- Text / GeoText minimal baseline: `TextDataset`, GeoText encode/decode, CRUD, `CreateTextDataset()`, and roundtrip fixture generation
- CAD minimal GeoHeader baseline: `CadDataset`, `CadPoint`, `CadLine`, `CadRegion`, CRUD, and `CreateCadDataset()`
- udbx4spec compliance coverage for `test_text`, `test_cad`, and cross-language roundtrip fixtures
- GAIA geometry codec for binary encoding/decoding
- System table DAOs (SmRegister, SmFieldInfo, geometry_columns, SmDataSourceInfo)
- Comprehensive error handling with typed errors
- GeoJSON-like geometry model
- CRUD operations for implemented dataset types
- Test suite with 76%+ coverage
- Documentation (README, AGENTS.md, CLAUDE.md compatibility entry)

### Features

- **Open/Close**: Open existing UDBX files and create new ones
- **Dataset Management**: List, get, and create datasets
- **Point Dataset**: 2D and 3D point features with full CRUD
- **Line Dataset**: MultiLineString features with full CRUD
- **Region Dataset**: MultiPolygon features with full CRUD
- **Tabular Dataset**: Non-spatial tables with full CRUD
- **Text Dataset**: GeoText features with minimal read/write CRUD
- **CAD Dataset**: Minimal GeoHeader `GeoPoint` / `GeoLine` / `GeoRegion` features with CRUD
- **Query Options**: Limit, offset, and ID-based filtering
- **Batch Operations**: InsertMany for efficient bulk inserts

### Technical

- SQLite-based storage using `github.com/mattn/go-sqlite3`
- Little-endian GAIA geometry encoding
- TDD development approach
- Table-driven tests with testify
- Type re-exports for convenient API access

### Known limitations

- Text / GeoText and CAD support are declared as minimal compliance baselines, not full compatibility with every complex SuperMap style or object variant.
- Stable T3 coverage currently targets approved `SampleData.udbx` source-derived fixtures and selected real sample integration tests.
- Network, Network3D, Model, and other unsupported dataset kinds are outside the `0.1.0` support scope.
