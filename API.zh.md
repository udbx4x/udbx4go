# API 文档

udbx4go 完整 API 参考。

[English](./API.md)

## 目录

- [DataSource](#datasource)
- [数据集类型](#数据集类型)
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

#### GetDataset

```go
func (ds *DataSource) GetDataset(name string) (dataset.Dataset, error)
```

根据名称获取数据集（通用接口）。

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

## 数据集类型

### TabularDataset

非空间数据集，用于纯属性数据。

#### 方法

##### GetByID

```go
func (d *TabularDataset) GetByID(id int) (*TabularRecord, error)
```

根据 ID 获取记录。

##### List

```go
func (d *TabularDataset) List(opts *QueryOptions) ([]*TabularRecord, error)
```

返回记录列表。

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

更新记录的属性。

##### Delete

```go
func (d *TabularDataset) Delete(id int) error
```

根据 ID 删除记录。

### PointDataset

二维点数据集。

#### 方法

##### GetByID

```go
func (d *PointDataset) GetByID(id int) (*Feature, error)
```

根据 ID 获取要素。

##### List

```go
func (d *PointDataset) List(opts *QueryOptions) ([]*Feature, error)
```

返回要素列表。

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

更新要素。

**示例：**
```go
changes := &udbx4go.FeatureChanges{
    Attributes: map[string]interface{}{
        "population": 22000000,
    },
}
err = pointDS.Update(1, changes)
```

##### Delete

```go
func (d *PointDataset) Delete(id int) error
```

根据 ID 删除要素。

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
    SRID        int        // 可选的 SRID
    BBox        []float64  // 可选的边界框
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
    BBox        []float64
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
    BBox        []float64
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

用于部分更新。

```go
type FeatureChanges struct {
    Geometry   Geometry
    Attributes map[string]interface{}
}
```

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
