# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Added `DataSource.QuerySpatial` and `GetSpatialQueryCapability` for Point, Line, Region, corresponding Z datasets, Text, and CAD.
- Added closed-interval MBR queries with stable `hasMore`, required-ID, strategy, and six-reason-code semantics from `udbx4spec`.
- Added verified RTree querying and DataSource-lifetime envelope caches; cache admission failures return the stable `envelope_cache_budget_exceeded` error.
- Added context-aware `ListContext` methods for spatial, Text, and CAD datasets.
- Added Text/CAD viewport querying through the public spatial-query contract, including envelope-cache filtering from `SmIndexKey`, payload decoding from `SmGeometry`, required IDs, context cancellation, and stable error reasons.
- Added CAD `SmIndexKey` writes and Text/CAD mutation-driven envelope-cache invalidation so create/update/delete operations cannot publish or reuse stale query state.
- Added Viewer-private non-spatial `bounded_sample` previews only for `envelope_cache_budget_exceeded` and `spatial_index_unavailable`, while retaining `degradedReason` in the Viewer DTO.
- Added Viewer viewport coordination, offscreen selection retention, deterministic stale-result probes, canvas-pixel blank-render gates, real-sample automatic tests, the envelope-cache PoC, and transactional macOS benchmark tooling.

### Changed

- The current envelope-cache defaults are 32 MiB per dataset and 64 MiB per open DataSource. They are measured SDK resource policies, not UDBX format limits.
- Tightened `QuerySpatial` to the `udbx4spec` contract: successful strategies are only `rtree` and `envelope_cache`; ordinary features must intersect the requested MBR, only required IDs may be offscreen, and `SpatialQueryResult` no longer carries degraded-result diagnostics.
- Viewer query concurrency is fixed at 1 by the 2026-07-23 packaged acceptance run; changing it requires a new candidate calibration and final rerun.

### Known limitations

- Coordinate projection and exact topology predicates are not implemented; spatial queries use closed-interval MBR intersection semantics.
- SDK, Viewer, frontend, specification, and benchmark workflow gates are automated. The 2026-07-23 packaged run completed all 30 rounds with deterministic stale-result and canvas-pixel gates, and the matching Text/CAD manual smoke completed 5/5; absolute performance remains specific to the tested Mac, system, samples, and workload.

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
