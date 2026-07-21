# udbx4spec 合规性检查报告

## 项目信息

| 项目 | 值 |
|------|-----|
| 项目名称 | udbx4go |
| 编程语言 | Go |
| 模块路径 | github.com/udbx4x/udbx4go |
| Go 版本 | 1.22 |
| 检查时间 | 2026-07-20 |
| 规范版本 | udbx4spec (当前) |

## 项目识别结果

✅ **确认为 UDBX 项目**

检测到的特征：
- 存在 `go.mod` 和 Go 源文件
- 存在 `DatasetKind` 类型定义
- 存在 `DataSource` 入口类
- 存在 `SmRegister` 系统表操作
- 存在 GAIA 几何编解码器

## 检查概览

| 检查项 | 状态 | 通过数/总数 | 百分比 |
|--------|------|-------------|--------|
| DatasetKind 完整性 | ✅ | 9/9 | 100% |
| FieldType 完整性 | ✅ | 14/14 | 100% |
| 几何类型完整性 | ✅ | 3/3 | 100% |
| 错误类型完整性 | ✅ | 5/5 | 100% |
| 数据集类完整性 | ✅ | 9/9 | 100% |
| DataSource API | ✅ | 16/16 | 100% |
| 视口空间查询契约 | ✅ | 1/1 | 100% |
| **总计** | **✅** | **57/57** | **100%** |

注意：本报告的接口存在性评分不代表所有真实 SuperMap UDBX 变体都已完全兼容。当前可执行合规覆盖已包含 2D/3D 矢量、Tabular、Text / GeoText 最小基线、CAD 最小 GeoHeader（`GeoPoint`、`GeoLine`、`GeoRegion`）读写闭环，以及 Go 运行时的视口空间查询。Java 和 TypeScript 当前只有 `udbx4spec` reference-only 契约，没有对应 SDK 运行时实现。

## 可执行合规测试

当前已接入 `../udbx4spec/compliance/roundtrip-matrix.md` 的 P0/P1 测试入口：

```bash
go test ./internal/codec -run Udbx4Spec -v
go test . -run Udbx4Spec -v
```

| 矩阵项 | 状态 | 覆盖内容 |
|--------|------|----------|
| R1 Golden decode | ✅ | 解码 `udbx4spec/compliance/golden-gaia-bytes/` 中的 2D/3D Point、MultiLineString、MultiPolygon fixture |
| R2 Golden encode | ✅ | 对解码后的 GeoJSON-like 几何重新编码，并与 Golden Bytes 做字节级一致性比较 |
| R3 Compliance read | ✅ | 打开 `udbx4spec/compliance/compliance.udbx`，校验 2D/3D 矢量、tabular、Text / GeoText 与 CAD 最小 GeoHeader 数据集的 kind、对象数、字段类型和代表性记录 |
| R4 单语言语义 roundtrip | ✅ | 读取覆盖 `point/line/region/pointZ/lineZ/regionZ/tabular/cad/text` 的 `compliance.udbx` 后通过 Go SDK 写出临时 UDBX，再重新打开并比较数据集、字段、对象数、几何和属性语义 |
| R5 跨语言语义 roundtrip | ✅ 三实现闭环 | 读取 `udbx4spec/compliance/roundtrip/udbx4ts-roundtrip.udbx`、`udbx4spec/compliance/roundtrip/udbx4j-roundtrip.udbx`，验证 Go 能读取 TypeScript 与 Java 写出的 UDBX；同时生成 `udbx4go-roundtrip.udbx`，供 `udbx4j`、`udbx4ts` 读取验证 |
| R6 Source-derived fixture | ✅ stable | 读取 `source-derived/sampledata/county-t/smid-1-smgeometry.bin`，验证真实 Text 中非 UTF-8 可读 `faceName` / `subText` 的容错解码行为；读取 `source-derived/sampledata/caddt/smid-1/16/63-smgeometry.bin`，验证真实 CAD Point / Line / Region 无样式 GeoHeader 解码行为；校验 `sampledata-3d-srid-zero-metadata` 的 stable manifest 与 3D metadata-json，并通过真实样本测试覆盖 `BaseMap_PZ` / `BaseMap_LZ` / `BaseMap_RZ`；Go 已统一为逐非法字节替换为 U+FFFD |
| R7 Spatial query contract | ✅ | `node --test ../udbx4spec/tools/spatial-query-contract.test.mjs` 校验 JSON Schema、TypeScript 和 Java reference-only 类型的字段、两种成功策略与六种原因码一致；Go SDK 已实现并由单元、race、真实样本和 PoC 自动测试覆盖 |

未完成项：

- 继续扩大真实 SuperMap UDBX 样本兼容范围，尤其是复杂 Text 样式和复杂 CAD 对象。
- T3 Source-derived fixture 当前为 `stable`，原始字节一致性、授权字段、生成工具字段和脱敏状态由 `udbx4spec/tools/check-fixtures.mjs` 校验；发布前必须运行 `make test-stable-t3`。
- 公开 API 最小稳定面已按 `udbx4spec/docs/08-api-stable-surface.md` 收敛：`GetDataset` / `GetByID` 未命中返回 not found error，`List` 默认按 `SmID` 升序，`Count` 读取物理表真实行数，`Update/Delete` 对缺失对象返回 not found，`Update` 对未知字段返回 not found。

## 详细检查结果

### 1. DatasetKind 完整性 ✅

| 规范值 | 数值 | 实现状态 | 说明 |
|--------|------|----------|------|
| `tabular` | 0 | ✅ | `DatasetKindTabular` |
| `point` | 1 | ✅ | `DatasetKindPoint` |
| `line` | 3 | ✅ | `DatasetKindLine` |
| `region` | 5 | ✅ | `DatasetKindRegion` |
| `text` | 7 | ✅ | `DatasetKindText` |
| `pointZ` | 101 | ✅ | `DatasetKindPointZ` |
| `lineZ` | 103 | ✅ | `DatasetKindLineZ` |
| `regionZ` | 105 | ✅ | `DatasetKindRegionZ` |
| `cad` | 149 | ✅ | `DatasetKindCAD` |

**附加功能：**
- `String()` 方法：返回规范字符串表示
- `FromDatasetKindString()`：从字符串解析
- `IsSpatial()`：判断是否空间数据集
- `Is3D()`：判断是否为 3D 数据集
- `GeometryType()`：返回 GAIA geoType
- `CoordDimension()`：返回坐标维度

### 2. FieldType 完整性 ✅

| 规范名 | 数值 | 实现状态 | SQLite 类型 | 说明 |
|--------|------|----------|-------------|------|
| `boolean` | 1 | ✅ | INTEGER | `FieldTypeBoolean` |
| `byte` | 2 | ✅ | INTEGER | `FieldTypeByte` |
| `int16` | 3 | ✅ | INTEGER | `FieldTypeInt16` |
| `int32` | 4 | ✅ | INTEGER | `FieldTypeInt32` |
| `int64` | 5 | ✅ | INTEGER | `FieldTypeInt64` |
| `single` | 6 | ✅ | REAL | `FieldTypeSingle` |
| `double` | 7 | ✅ | REAL | `FieldTypeDouble` |
| `date` | 8 | ✅ | TEXT | `FieldTypeDate` |
| `binary` | 9 | ✅ | BLOB | `FieldTypeBinary` |
| `geometry` | 10 | ✅ | BLOB | `FieldTypeGeometry` |
| `char` | 11 | ✅ | TEXT | `FieldTypeChar` |
| `ntext` | 127 | ✅ | TEXT | `FieldTypeNText` |
| `text` | 128 | ✅ | TEXT | `FieldTypeText` |
| `time` | 16 | ✅ | TEXT | `FieldTypeTime` |

**附加功能：**
- `String()` 方法：返回规范字符串表示
- `FromFieldTypeString()`：从字符串解析
- `SQLiteType()`：返回 SQLite 存储类型
- `GoType()`：返回 Go 类型描述

### 3. 几何类型完整性 ✅

| 规范类型 | 实现类型 | 状态 | 方法 |
|----------|----------|------|------|
| `PointGeometry` | `PointGeometry` | ✅ | `GeometryType()`, `GetSRID()`, `HasZ()`, `GetBBox()`, `X()`, `Y()`, `Z()` |
| `MultiLineStringGeometry` | `MultiLineStringGeometry` | ✅ | `GeometryType()`, `GetSRID()`, `HasZ()`, `GetBBox()` |
| `MultiPolygonGeometry` | `MultiPolygonGeometry` | ✅ | `GeometryType()`, `GetSRID()`, `HasZ()`, `GetBBox()` |

### 4. 错误类型完整性 ✅

| 规范错误 | 实现状态 | 说明 |
|----------|----------|------|
| `UdbxError` (interface) | ✅ | `UdbxError` interface |
| `UdbxFormatError` | ✅ | `FormatError()` |
| `UdbxNotFoundError` | ✅ | `NotFoundError()`, `NotFoundErrorf()` |
| `UdbxUnsupportedError` | ✅ | `UnsupportedError()` |
| `UdbxConstraintError` | ✅ | `ConstraintError()` |
| `UdbxIOError` | ✅ | `IOError()`, `IOErrorf()` |

**哨兵错误：**
| 哨兵 | 实现状态 |
|------|----------|
| `ErrNotFound` | ✅ |
| `ErrFormat` | ✅ |
| `ErrUnsupported` | ✅ |
| `ErrConstraint` | ✅ |
| `ErrIO` | ✅ |

### 5. 数据集类完整性 ✅

| 规范类名 | 实现状态 | 文件 | 方法 |
|----------|----------|------|------|
| `TabularDataset` | ✅ | `tabular.go` | `GetByID()`, `List()`, `Insert()`, `InsertMany()`, `Update()`, `Delete()` |
| `PointDataset` | ✅ | `point.go` | `GetByID()`, `List()`, `Insert()`, `InsertMany()`, `Update()`, `Delete()` |
| `LineDataset` | ✅ | `line.go` | `GetByID()`, `List()`, `Insert()`, `InsertMany()`, `Update()`, `Delete()` |
| `RegionDataset` | ✅ | `region.go` | `GetByID()`, `List()`, `Insert()`, `InsertMany()`, `Update()`, `Delete()` |
| `PointZDataset` | ✅ | `pointz.go` | 继承自 PointDataset |
| `LineZDataset` | ✅ | `linez.go` | 继承自 LineDataset |
| `RegionZDataset` | ✅ | `regionz.go` | 继承自 RegionDataset |
| `TextDataset` | ✅ | `text.go` | `GetByID()`, `List()`, `Insert()`, `InsertMany()`, `Update()`, `Delete()` |
| `CadDataset` | ✅ | `cad.go` | `GetByID()`, `List()`, `Insert()`, `InsertMany()`, `Update()`, `Delete()` |

### 6. DataSource API ✅

#### 6.1 生命周期方法 ✅

| 规范方法 | 实现状态 | 签名 |
|----------|----------|------|
| `open()` | ✅ | `Open(path string) (*DataSource, error)` |
| `create()` | ✅ | `Create(path string) (*DataSource, error)` |
| `close()` | ✅ | `Close() error` |

#### 6.2 数据集查询方法 ✅

| 规范方法 | 实现状态 | 签名 |
|----------|----------|------|
| `listDatasets()` | ✅ | `ListDatasets() ([]*DatasetInfo, error)` |
| `getDataset(name)` | ✅ | `GetDataset(name string) (Dataset, error)` |
| `getSpatialQueryCapability(ctx, name)` | ✅ | `GetSpatialQueryCapability(context.Context, string) (*SpatialQueryCapability, error)` |
| `querySpatial(ctx, name, options)` | ✅ | `QuerySpatial(context.Context, string, SpatialQueryOptions) (*SpatialQueryResult, error)` |

#### 6.3 类型专用获取方法 ✅

| 规范方法 | 实现状态 | 签名 |
|----------|----------|------|
| `getTabularDataset()` | ✅ | `GetTabularDataset(name string) (*TabularDataset, error)` |
| `getPointDataset()` | ✅ | `GetPointDataset(name string) (*PointDataset, error)` |
| `getLineDataset()` | ✅ | `GetLineDataset(name string) (*LineDataset, error)` |
| `getRegionDataset()` | ✅ | `GetRegionDataset(name string) (*RegionDataset, error)` |
| `getTextDataset()` | ✅ | `GetTextDataset(name string) (*TextDataset, error)` |
| `getCadDataset()` | ✅ | `GetCadDataset(name string) (*CadDataset, error)` |

#### 6.4 数据集创建方法 ✅

| 规范方法 | 实现状态 | 签名 |
|----------|----------|------|
| `createTabularDataset()` | ✅ | `CreateTabularDataset(name string, fields []*FieldInfo) (*TabularDataset, error)` |
| `createPointDataset()` | ✅ | `CreatePointDataset(name string, srid int, fields []*FieldInfo) (*PointDataset, error)` |
| `createLineDataset()` | ✅ | `CreateLineDataset(name string, srid int, fields []*FieldInfo) (*LineDataset, error)` |
| `createRegionDataset()` | ✅ | `CreateRegionDataset(name string, srid int, fields []*FieldInfo) (*RegionDataset, error)` |
| `createPointZDataset()` | ✅ | `CreatePointZDataset(name string, srid int, fields []*FieldInfo) (*PointZDataset, error)` |
| `createLineZDataset()` | ✅ | `CreateLineZDataset(name string, srid int, fields []*FieldInfo) (*LineZDataset, error)` |
| `createRegionZDataset()` | ✅ | `CreateRegionZDataset(name string, srid int, fields []*FieldInfo) (*RegionZDataset, error)` |
| `createTextDataset()` | ✅ | `CreateTextDataset(name string, srid int, fields []*FieldInfo) (*TextDataset, error)` |
| `createCadDataset()` | ✅ | `CreateCadDataset(name string, fields []*FieldInfo) (*CadDataset, error)` |

### 7. 其他类型检查 ✅

| 规范类型 | 实现状态 | 说明 |
|----------|----------|------|
| `Feature` | ✅ | `Feature` struct |
| `TabularRecord` | ✅ | `TabularRecord` struct |
| `DatasetInfo` | ✅ | `DatasetInfo` struct |
| `FieldInfo` | ✅ | `FieldInfo` struct |
| `QueryOptions` | ✅ | `QueryOptions` struct |
| `BoundingBox` / `SpatialQueryOptions` / `SpatialQueryResult` | ✅ | Go 运行时实现；闭区间 MBR、`limit + 1`、`requiredIds` 和 `hasMore` 语义与规范一致；成功结果无 `DegradedReason` |
| `SpatialQueryStrategy` | ✅ | SDK 成功策略仅为 `rtree`、`envelope_cache` |
| `SpatialQueryReason` | ✅ | 六个稳定原因码 |

## 当前实现边界

| 类别 | 状态 | 后续方向 |
|------|------|----------|
| Text / GeoText | 已完成最小合规基线 | 扩展复杂样式、复杂文本对象和更多真实样本 |
| CAD GeoHeader | 已完成最小合规基线 | 扩展复杂 CAD 类型、样式和混合对象 |
| 视口空间查询 | Go 已实现并通过 SDK/自动测试；Java/TypeScript reference-only | Text/CAD 视口查询、投影转换和精确拓扑谓词；严格 stale/canvas 门禁的 macOS 打包运行需要重新验收 |

## 命名一致性检查

| 规范名 | 当前实现 | 状态 | 说明 |
|--------|----------|------|------|
| `getById` | `GetByID` | ✅ | Go 使用 ID 而非 Id，符合 Go 惯例 |
| `list` | `List` | ✅ | 一致 |
| `insert` | `Insert` | ✅ | 一致 |
| `insertMany` | `InsertMany` | ✅ | 一致 |
| `update` | `Update` | ✅ | 一致 |
| `delete` | `Delete` | ✅ | 一致 |
| `count` | `Count() (int, error)` | ✅ | BaseDataset 提供，语义已统一为物理表实际计数 |

## 规范符合度评分

| 维度 | 得分 | 说明 |
|------|------|------|
| 类型系统 | 100% | DatasetKind、FieldType、Geometry 完整 |
| 错误处理 | 100% | 所有错误类型和哨兵错误已实现 |
| 核心功能 | 100% | 当前最小合规基线已覆盖 Text 与 CAD |
| **总体** | **100%** | 当前 udbx4spec 最小合规基线通过 |

## 建议

### 后续建议

1. **考虑添加 AsyncIterable 支持**
   - 规范中有 `iterate()` 方法返回 AsyncIterable
   - Go 中可使用 channel 或 iterator 模式

2. **完善文档**
   - 已符合规范的模块可添加 "spec-compliant" 注释

## 结论

**udbx4go 项目已通过当前 udbx4spec 最小合规基线。**

主要优点：
- DatasetKind 和 FieldType 实现完整
- 几何模型符合 GeoJSON-like 规范
- 错误处理体系完善
- API 命名符合 Go 语言惯例
- Text / GeoText 最小基线已纳入合规闭环
- CAD 最小 GeoHeader 基线已纳入合规闭环
- Go 视口空间查询已实现；两种 SDK 成功策略、普通要素 MBR 相交、仅 required ID 可在视口外、预算超限错误和六个原因码均已纳入合规闭环
- Viewer 私有 `bounded_sample` 是预算错误或 Text/CAD 路径的非空间有界预览，不属于 SDK 成功结果；Viewer DTO 可保留 `degradedReason`

需要改进：
- 扩展真实 SuperMap UDBX 样本文本和 CAD 兼容范围
- 使用严格契约、确定性 stale probe 和 canvas 像素门禁重新执行 macOS 打包三场景验收；证据按人工 1-4、自动化故障注入 5、真实切换加自动化生命周期 6 分类，当前固定并发保持 1

建议后续以真实样本兼容和发布治理为重点继续推进。
