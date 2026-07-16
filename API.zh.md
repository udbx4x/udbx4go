# API 文档

udbx4go 完整 API 参考。

注意：当前数据集具体类型来自内部实现包。外部调用者可以直接使用 `DataSource` 方法返回的值并调用其方法，但不应导入内部包，也不应把这些具体类型名称视为稳定公开 API。

[English](./API.md)

## 目录

- [DataSource](#datasource)
- [视口空间查询](#视口空间查询)
- [数据集类型](#数据集类型)
  - [数据集稳定语义](#数据集稳定语义)
- [几何类型](#几何类型)
- [要素和记录](#要素和记录)
- [查询选项](#查询选项)
- [错误处理](#错误处理)

## DataSource

`DataSource` 是操作 UDBX 文件的入口点。

### 函数

#### Open

```go
func Open(path string) (*DataSource, error)
```

打开已有的 UDBX 文件。

**示例：**
```go
ds, err := udbx4go.Open("data.udbx")
if err != nil {
    log.Fatal(err)
}
defer ds.Close()
```

#### Create

```go
func Create(path string) (*DataSource, error)
```

创建新的 UDBX 文件。

**示例：**
```go
ds, err := udbx4go.Create("newdata.udbx")
if err != nil {
    log.Fatal(err)
}
defer ds.Close()
```

### 方法

#### Close

```go
func (ds *DataSource) Close() error
```

关闭数据源并释放资源。

#### ListDatasets

```go
func (ds *DataSource) ListDatasets() ([]*DatasetInfo, error)
```

返回数据源中所有数据集的列表。

**示例：**
```go
datasets, err := ds.ListDatasets()
for _, info := range datasets {
    fmt.Printf("%s: %s\n", info.Name, info.Kind)
}
```

#### GetSpatialQueryCapability

```go
func (ds *DataSource) GetSpatialQueryCapability(
    ctx context.Context,
    datasetName string,
) (*SpatialQueryCapability, error)
```

报告数据集是否支持视口查询、是否存在经过结构校验的 RTree，以及是否可以尝试内存回退路径。

#### QuerySpatial

```go
func (ds *DataSource) QuerySpatial(
    ctx context.Context,
    datasetName string,
    options SpatialQueryOptions,
) (*SpatialQueryResult, error)
```

按视口 MBR 查询 Point、Line、Region、PointZ、LineZ 或 RegionZ 要素。完整示例和结果语义见[视口空间查询](#视口空间查询)。

#### GetDataset

```go
func (ds *DataSource) GetDataset(name string) (dataset.Dataset, error)
```

根据名称通过内部通用数据集接口获取数据集。外部调用者通常应优先使用 `GetPointDataset` 等类型专用获取方法。

#### GetTabularDataset

```go
func (ds *DataSource) GetTabularDataset(name string) (*TabularDataset, error)
```

根据名称获取属性表数据集。

#### GetPointDataset

```go
func (ds *DataSource) GetPointDataset(name string) (*PointDataset, error)
```

根据名称获取点数据集。

#### GetLineDataset

```go
func (ds *DataSource) GetLineDataset(name string) (*LineDataset, error)
```

根据名称获取线数据集。

#### GetRegionDataset

```go
func (ds *DataSource) GetRegionDataset(name string) (*RegionDataset, error)
```

根据名称获取面数据集。

### 数据集创建

#### CreateTabularDataset

```go
func (ds *DataSource) CreateTabularDataset(
    name string,
    fields []*FieldInfo,
) (*TabularDataset, error)
```

创建新的属性表（非空间）数据集。

**示例：**
```go
fields := []*udbx4go.FieldInfo{
    {Name: "code", FieldType: udbx4go.FieldTypeText, Required: true},
    {Name: "value", FieldType: udbx4go.FieldTypeDouble, Nullable: true},
}

tabularDS, err := ds.CreateTabularDataset("attributes", fields)
```

#### CreatePointDataset

```go
func (ds *DataSource) CreatePointDataset(
    name string,
    srid int,
    fields []*FieldInfo,
) (*PointDataset, error)
```

创建新的二维点数据集。

**示例：**
```go
fields := []*udbx4go.FieldInfo{
    {Name: "name", FieldType: udbx4go.FieldTypeText},
    {Name: "population", FieldType: udbx4go.FieldTypeInt32},
}

pointDS, err := ds.CreatePointDataset("cities", 4326, fields)
```

#### CreateLineDataset

```go
func (ds *DataSource) CreateLineDataset(
    name string,
    srid int,
    fields []*FieldInfo,
) (*LineDataset, error)
```

创建新的二维线数据集。

#### CreateRegionDataset

```go
func (ds *DataSource) CreateRegionDataset(
    name string,
    srid int,
    fields []*FieldInfo,
) (*RegionDataset, error)
```

创建新的二维面数据集。

#### CreatePointZDataset

```go
func (ds *DataSource) CreatePointZDataset(
    name string,
    srid int,
    fields []*FieldInfo,
) (*PointZDataset, error)
```

创建新的三维点数据集。

#### CreateLineZDataset

```go
func (ds *DataSource) CreateLineZDataset(
    name string,
    srid int,
    fields []*FieldInfo,
) (*LineZDataset, error)
```

创建新的三维线数据集。

#### CreateRegionZDataset

```go
func (ds *DataSource) CreateRegionZDataset(
    name string,
    srid int,
    fields []*FieldInfo,
) (*RegionZDataset, error)
```

创建新的三维面数据集。

## 视口空间查询

以下程序打开 UDBX 文件，执行可取消的视口查询，并输出 SDK 返回的执行事实：

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/udbx4x/udbx4go"
)

func main() {
    ds, err := udbx4go.Open("henan.udbx")
    if err != nil {
        log.Fatal(err)
    }
    defer ds.Close()

    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()

    result, err := ds.QuerySpatial(ctx, "weibo", udbx4go.SpatialQueryOptions{
        Bounds: udbx4go.BoundingBox{
            MinX: 113.5,
            MinY: 34.5,
            MaxX: 114.0,
            MaxY: 35.0,
        },
        Limit:       1000,
        RequiredIDs: []int{12345},
    })
    if err != nil {
        if reason, ok := udbx4go.SpatialQueryReasonOf(err); ok {
            log.Fatalf("空间查询失败（%s）：%v", reason, err)
        }
        log.Fatal(err)
    }

    fmt.Printf("features=%d hasMore=%t strategy=%s degradedReason=%s\n",
        len(result.Features), result.HasMore, result.Strategy, result.DegradedReason)
}
```

### 查询语义

- `Bounds` 坐标必须是有限数值且顺序合法。MBR 相交使用闭区间，因此与查询边界接触的要素也会命中。
- SDK 最多读取 `Limit + 1` 个普通视口候选；额外一条只用于设置 `HasMore`，不会进入返回结果。
- `RequiredIDs` 去重后，在对象存在时追加到结果，即使对象位于视口外。它们不占用 `Limit`，也不影响 `HasMore`，所以 `len(Features)` 可以大于 `Limit`。
- `Features` 是视口 MBR 命中对象和实际存在的必含对象的去重并集；必含对象不保证与 `QueriedBounds` 相交。
- 当前只提供 MBR 过滤，不等价于 `Intersects`、`Contains`、`Within` 等精确拓扑谓词。

`Strategy` 记录实际执行路径：

| 值 | 含义 |
|----|------|
| `rtree` | 使用经过结构校验的 UDBX RTree 生成视口候选。 |
| `envelope_cache` | 数据集没有可用 RTree，使用内存 GAIA 包络缓存生成候选。 |
| `bounded_sample` | 缓存准入超过当前资源策略，返回有界采样。 |

非降级结果的 `DegradedReason` 为空。错误和降级结果使用以下六个稳定原因码：

| 值 | 含义 |
|----|------|
| `invalid_viewport` | 边界框、limit 或必含 ID 非法。 |
| `spatial_index_unavailable` | 必需的空间元数据或可用索引路径不存在。 |
| `envelope_cache_budget_exceeded` | 构建完整包络缓存超过当前资源策略。 |
| `query_timeout` | context 被取消或超过截止时间。 |
| `corrupt_geometry` | GAIA 头部或完整几何损坏。 |
| `unsupported_dataset_kind` | 数据集类型不在视口查询范围内。 |

包络缓存只属于当前打开的一个 `DataSource`，不会写入 UDBX 文件，也不会跨进程共享；`Close` 会释放缓存。当前默认策略为单数据集 32 MiB、单个 `DataSource` 合计 64 MiB，构建超时 500 ms。这些数值是基于测量的 SDK 当前资源默认值，可以随实测调整，不是 UDBX 格式限制。本版本的 Text、CAD 和 Tabular 数据集不支持 `QuerySpatial`；Text 和 CAD 继续通过有上界的 `List`/`ListContext` 预览路径读取。

## 数据集类型

### 数据集稳定语义

所有数据集 API 遵循 `udbx4spec/docs/08-api-stable-surface.md`：

- `GetByID(id)` 在对象不存在时返回 `nil, err`，且 `udbx4go.IsNotFound(err)` 必须为 `true`。
- `List(opts)` 默认按 `SmID` 升序返回；`QueryOptions.IDs` 只过滤结果集合，不改变排序语义。
- `Count()` 读取物理表真实行数，不以 `SmRegister.SmObjectCount` 缓存为准。
- `Update(id, ...)` 和 `Delete(id)` 在目标对象不存在时返回 not found error。
- 更新未知字段时返回 not found 或约束类错误，不得静默忽略。

Point、Line、Region、Text 和 CAD 数据集还提供：

```go
func (d *PointDataset) ListContext(ctx context.Context, opts *QueryOptions) ([]*Feature, error)
```

对应具体数据集类型的方法形状相同。`List` 等价于使用 `context.Background()` 调用 `ListContext`。调用方需要通过截止时间或取消来停止 SQLite 遍历和几何解码时，应使用 `ListContext`。取消映射为 `query_timeout` 空间原因码，损坏几何映射为 `corrupt_geometry`。Tabular 当前只提供 `List`。

### TabularDataset

非空间数据集，用于纯属性数据。

#### 方法

##### GetByID

```go
func (d *TabularDataset) GetByID(id int) (*TabularRecord, error)
```

根据 ID 获取记录。记录不存在时返回 `nil, err`，且 `udbx4go.IsNotFound(err)` 为 `true`。

##### List

```go
func (d *TabularDataset) List(opts *QueryOptions) ([]*TabularRecord, error)
```

返回记录列表，默认按 `SmID` 升序排列。

##### Insert

```go
func (d *TabularDataset) Insert(record *TabularRecord) error
```

插入新记录。

##### InsertMany

```go
func (d *TabularDataset) InsertMany(records []*TabularRecord) error
```

在事务中插入多条记录。

##### Update

```go
func (d *TabularDataset) Update(id int, attributes map[string]interface{}) error
```

更新记录的属性。记录不存在时返回 not found error。

##### Delete

```go
func (d *TabularDataset) Delete(id int) error
```

根据 ID 删除记录。记录不存在时返回 not found error。

### PointDataset

二维点数据集。

#### 方法

##### GetByID

```go
func (d *PointDataset) GetByID(id int) (*Feature, error)
```

根据 ID 获取要素。要素不存在时返回 `nil, err`，且 `udbx4go.IsNotFound(err)` 为 `true`。

##### List

```go
func (d *PointDataset) List(opts *QueryOptions) ([]*Feature, error)
```

返回要素列表，默认按 `SmID` 升序排列。

##### Insert

```go
func (d *PointDataset) Insert(feature *Feature) error
```

插入新的点要素。

**示例：**
```go
feature := &udbx4go.Feature{
    ID: 1,
    Geometry: &udbx4go.PointGeometry{
        Type:        "Point",
        Coordinates: []float64{116.4, 39.9},
    },
    Attributes: map[string]interface{}{
        "name": "北京",
    },
}
err = pointDS.Insert(feature)
```

##### InsertMany

```go
func (d *PointDataset) InsertMany(features []*Feature) error
```

在事务中插入多个要素。

##### Update

```go
func (d *PointDataset) Update(id int, changes *FeatureChanges) error
```

使用公开的 `udbx4go.FeatureChanges` 类型更新要素。

要素不存在时返回 not found error。

**示例：**
```go
err = pointDS.Update(1, &udbx4go.FeatureChanges{
    Geometry: &udbx4go.PointGeometry{
        Type:        "Point",
        Coordinates: []float64{121.5, 31.2},
    },
    Attributes: map[string]interface{}{
        "population": 26320000,
    },
})
```

##### Delete

```go
func (d *PointDataset) Delete(id int) error
```

根据 ID 删除要素。要素不存在时返回 not found error。

### LineDataset

二维线（多线串）数据集。方法与 PointDataset 相同，但要求使用 MultiLineStringGeometry。

### RegionDataset

二维面（多多边形）数据集。方法与 PointDataset 相同，但要求使用 MultiPolygonGeometry。

### PointZDataset、LineZDataset、RegionZDataset

三维变体，支持 Z 坐标。

## 几何类型

### PointGeometry

```go
type PointGeometry struct {
    Type        string
    Coordinates []float64  // [x, y] 表示二维，[x, y, z] 表示三维
    SRID        int        // 可选 SRID，未设置时为 0
    HasZValue   bool       // 可选的显式 Z 标记
    BBox        []float64  // 可选边界框 [minX, minY, maxX, maxY]
    GeoType     int        // 可选 GAIA geoType
}
```

**方法：**
- `GeometryType() string` - 返回 "Point"
- `GetSRID() int` - 返回 SRID 或 0
- `HasZ() bool` - 如果有 Z 坐标返回 true
- `GetBBox() []float64` - 返回边界框
- `X() float64` - 返回 X 坐标
- `Y() float64` - 返回 Y 坐标
- `Z() float64` - 返回 Z 坐标（如果是二维则返回 0）

### MultiLineStringGeometry

```go
type MultiLineStringGeometry struct {
    Type        string
    Coordinates [][][]float64  // 线串数组
    SRID        int
    HasZValue   bool
    BBox        []float64
    GeoType     int
}
```

**方法：**
- `GeometryType() string` - 返回 "MultiLineString"
- `GetSRID() int`
- `HasZ() bool`
- `GetBBox() []float64`

### MultiPolygonGeometry

```go
type MultiPolygonGeometry struct {
    Type        string
    Coordinates [][][][]float64  // 多边形数组，每个多边形包含环
    SRID        int
    HasZValue   bool
    BBox        []float64
    GeoType     int
}
```

**方法：**
- `GeometryType() string` - 返回 "MultiPolygon"
- `GetSRID() int`
- `HasZ() bool`
- `GetBBox() []float64`

## 要素和记录

### Feature

空间要素，包含几何和属性。

```go
type Feature struct {
    ID         int
    Geometry   Geometry
    Attributes map[string]interface{}
}
```

### TabularRecord

非空间记录，仅包含属性。

```go
type TabularRecord struct {
    ID         int
    Attributes map[string]interface{}
}
```

### FeatureChanges

矢量要素更新使用的公开变更对象。

```go
type FeatureChanges struct {
    Geometry   Geometry
    Attributes map[string]interface{}
}
```

通过 `udbx4go.FeatureChanges` 调用矢量数据集 `Update` 方法。`Geometry` 和 `Attributes` 均为可选；两者都为空时，实现可以将调用视为无操作。

## 查询选项

```go
type QueryOptions struct {
    IDs    []int  // 按特定 ID 过滤
    Limit  int    // 最大结果数
    Offset int    // 跳过的结果数（在 SQLite 中需要 Limit）
}
```

**示例：**

```go
// 获取前 10 个要素
opts := &udbx4go.QueryOptions{Limit: 10}
features, err := dataset.List(opts)

// 获取 ID 为 1、3、5 的要素
opts := &udbx4go.QueryOptions{IDs: []int{1, 3, 5}}
features, err := dataset.List(opts)

// 分页（跳过前 20 个，获取后 10 个）
opts := &udbx4go.QueryOptions{Limit: 10, Offset: 20}
features, err := dataset.List(opts)
```

## 错误处理

### 错误类型

所有错误都实现了 `UdbxError` 接口：

```go
type UdbxError interface {
    error
    Code() ErrorCode
}
```

### 错误类别

| 函数 | 说明 |
|------|------|
| `IsFormatError(err error)` | 无效的 UDBX 格式 |
| `IsNotFound(err error)` | 数据集或要素不存在 |
| `IsUnsupported(err error)` | 不支持的操作 |
| `IsConstraintViolation(err error)` | 数据约束冲突 |
| `IsIOError(err error)` | 文件 I/O 错误 |

### 哨兵错误

| 错误 | 说明 |
|------|------|
| `ErrNotFound` | 不存在错误 |
| `ErrFormat` | 格式错误 |
| `ErrUnsupported` | 不支持错误 |
| `ErrConstraint` | 约束错误 |
| `ErrIO` | I/O 错误 |

### 示例

```go
// 检查错误类型
feature, err := dataset.GetByID(999)
if err != nil {
    if udbx4go.IsNotFound(err) {
        fmt.Println("要素不存在")
    } else {
        log.Fatal(err)
    }
}

// 与 errors.Is 一起使用
if errors.Is(err, udbx4go.ErrNotFound) {
    // 处理不存在的情况
}
```

## 常量参考

### DatasetKind

```go
const (
    DatasetKindTabular DatasetKind = 0   // 属性表
    DatasetKindPoint   DatasetKind = 1   // 二维点
    DatasetKindLine    DatasetKind = 3   // 二维线
    DatasetKindRegion  DatasetKind = 5   // 二维面
    DatasetKindText    DatasetKind = 7   // 文本
    DatasetKindPointZ  DatasetKind = 101 // 三维点
    DatasetKindLineZ   DatasetKind = 103 // 三维线
    DatasetKindRegionZ DatasetKind = 105 // 三维面
    DatasetKindCAD     DatasetKind = 149 // CAD
)
```

### FieldType

```go
const (
    FieldTypeBoolean  FieldType = 1   // 布尔
    FieldTypeByte     FieldType = 2   // 字节
    FieldTypeInt16    FieldType = 3   // 16位整数
    FieldTypeInt32    FieldType = 4   // 32位整数
    FieldTypeInt64    FieldType = 5   // 64位整数
    FieldTypeSingle   FieldType = 6   // 单精度浮点
    FieldTypeDouble   FieldType = 7   // 双精度浮点
    FieldTypeDate     FieldType = 8   // 日期
    FieldTypeBinary   FieldType = 9   // 二进制
    FieldTypeGeometry FieldType = 10  // 几何
    FieldTypeChar     FieldType = 11  // 定长字符
    FieldTypeNText    FieldType = 127 // Unicode 文本
    FieldTypeText     FieldType = 128 // 文本
    FieldTypeTime     FieldType = 16  // 时间
)
```
