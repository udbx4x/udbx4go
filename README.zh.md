# udbx4go

[![Go Reference](https://pkg.go.dev/badge/github.com/udbx4x/udbx4go.svg)](https://pkg.go.dev/github.com/udbx4x/udbx4go)
[![Go Report Card](https://goreportcard.com/badge/github.com/udbx4x/udbx4go)](https://goreportcard.com/report/github.com/udbx4x/udbx4go)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](./)
[![Coverage](https://img.shields.io/badge/coverage-76.7%25-yellowgreen)](./)

UDBX（通用空间数据库扩展）读写库的 Go 语言实现。UDBX 是一种基于 SQLite 的空间数据格式，当前 Go SDK 支持属性表以及点、线、面、三维点、三维线、三维面、文本和 CAD 数据集。

[English](./README.md)

## 特性

- ✅ 核心 UDBX 读写能力已实现，包含 Text / GeoText 与 CAD 最小 GeoHeader 基线。
- ✅ 已实现数据集类型：点、线、面、三维点、三维线、三维面、属性表、文本、CAD。
- ✅ `TextDataset` 支持最小 GeoText 读写 CRUD；`CadDataset` 支持最小 GeoHeader `GeoPoint` / `GeoLine` / `GeoRegion`。
- ✅ Point、Line、Region、对应 Z 类型、Text 与 CAD 支持带 context 的视口 MBR 查询，并提供经过校验的 RTree 和包络缓存策略。
- ✅ 14 种字段类型，支持正确的类型映射
- ✅ 类 GeoJSON 几何模型
- ✅ 流式和批量操作
- ✅ 跨语言兼容性（udbx4j、udbx4ts）
- ✅ 完善的错误处理
- ✅ TDD 开发，测试覆盖率 76%+

## 安装

```bash
go get github.com/udbx4x/udbx4go
```

**注意**：本包需要 CGO，因为它使用了 `github.com/mattn/go-sqlite3`。请确保已安装 C 编译器。

## 快速开始

### 打开已有的 UDBX 文件

```go
package main

import (
    "log"
    "github.com/udbx4x/udbx4go"
)

func main() {
    // 打开已有的 UDBX 文件
    ds, err := udbx4go.Open("data.udbx")
    if err != nil {
        log.Fatal(err)
    }
    defer ds.Close()

    // 列出所有数据集
    datasets, err := ds.ListDatasets()
    if err != nil {
        log.Fatal(err)
    }
    for _, info := range datasets {
        log.Printf("数据集: %s (类型: %s)", info.Name, info.Kind)
    }

    // 获取点数据集
    pointDataset, err := ds.GetPointDataset("cities")
    if err != nil {
        log.Fatal(err)
    }

    // 查询要素
    features, err := pointDataset.List(&udbx4go.QueryOptions{Limit: 10})
    if err != nil {
        log.Fatal(err)
    }
    for _, f := range features {
        log.Printf("要素 %d: %v", f.ID, f.Attributes["name"])
    }
}
```

### 视口空间查询

`DataSource.QuerySpatial` 为地图视口返回稳定、有上界的要素集合：

```go
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()

result, err := ds.QuerySpatial(ctx, "weibo", udbx4go.SpatialQueryOptions{
    Bounds: udbx4go.BoundingBox{MinX: 113.5, MinY: 34.5, MaxX: 114.0, MaxY: 35.0},
    Limit: 1000,
    RequiredIDs: []int{12345},
})
```

MBR 相交采用闭区间，所有普通要素都必须与请求范围 MBR 相交。`HasMore` 只描述普通视口命中对象；必含 ID 会去重，不占用 `Limit`，也是唯一可以保留在视口外的要素。SDK 成功结果的策略只有 `rtree` 或 `envelope_cache`，`SpatialQueryResult` 没有 `DegradedReason` 字段。包络缓存只在当前 `DataSource` 生命周期存在，调用 `Close` 时释放。

当前单数据集 32 MiB、单个 `DataSource` 合计 64 MiB 是经过测量的缓存资源默认策略，按“每数据集约 4 MiB 固定 charge + 每 capacity entry 约 80 bytes”的稳定 RSS 模型计费，不是对象数或 UDBX 格式限制。完整缓存超出该策略时，SDK 返回 `envelope_cache_budget_exceeded` 错误。Text 与 CAD 使用 `SmIndexKey` 过滤包络候选，再从 `SmGeometry` 解码命中的业务对象；该路径要求存在包络列和有效空间元数据。缺少 `SmIndexKey` 或有效 `geometry_columns` 注册的 legacy CAD 不会通过非空间路径伪装成查询成功，而是报告 `spatial_index_unavailable`。请求未提供 viewport 时，Viewer 通过 `ListContext` 按 ID 读取私有的初始有界 `bounded_sample`，`QueriedBounds` 和 `degradedReason` 均为空，声明 extent 只作为自动定位提示。采样诊断取决于该有界读取的真实 `HasMore` 和顶点预算截断，不使用 `SmObjectCount` 推断。请求提供 viewport 时，Viewer 调用 `QuerySpatial`：正常结果使用 `rtree` 或 `envelope_cache`；只有 `envelope_cache_budget_exceeded` 或 `spatial_index_unavailable` 才触发携带对应 `degradedReason` 的私有有界 fallback。`bounded_sample` 和 `degradedReason` 是 Viewer DTO 概念，不属于 SDK 成功策略或 `SpatialQueryResult` 字段。可运行完整程序、六个原因码和 `ListContext` 取消语义见 [API.zh.md](./API.zh.md#视口空间查询)。

### 创建新的 UDBX 文件

```go
package main

import (
    "log"
    "github.com/udbx4x/udbx4go"
)

func main() {
    // 创建新的 UDBX 文件
    ds, err := udbx4go.Create("newdata.udbx")
    if err != nil {
        log.Fatal(err)
    }
    defer ds.Close()

    // 创建点数据集，带自定义字段
    fields := []*udbx4go.FieldInfo{
        {Name: "name", FieldType: udbx4go.FieldTypeText, Nullable: true},
        {Name: "population", FieldType: udbx4go.FieldTypeInt32, Nullable: true},
    }

    pointDS, err := ds.CreatePointDataset("cities", 4326, fields)
    if err != nil {
        log.Fatal(err)
    }

    // 插入要素
    feature := &udbx4go.Feature{
        ID: 1,
        Geometry: &udbx4go.PointGeometry{
            Type:        "Point",
            Coordinates: []float64{116.4, 39.9},
        },
        Attributes: map[string]interface{}{
            "name":       "北京",
            "population": 21540000,
        },
    }

    if err := pointDS.Insert(feature); err != nil {
        log.Fatal(err)
    }
}
```

## CRUD 操作

### 点数据集

```go
// 根据 ID 获取
feature, err := pointDS.GetByID(1)
if err != nil {
    if udbx4go.IsNotFound(err) {
        log.Println("要素不存在")
    } else {
        log.Fatal(err)
    }
}

// 插入
newFeature := &udbx4go.Feature{
    ID: 2,
    Geometry: &udbx4go.PointGeometry{
        Type:        "Point",
        Coordinates: []float64{121.5, 31.2},
    },
    Attributes: map[string]interface{}{
        "name":       "上海",
        "population": 26320000,
    },
}
err = pointDS.Insert(newFeature)

// 更新
err = pointDS.Update(2, &udbx4go.FeatureChanges{
    Geometry: &udbx4go.PointGeometry{
        Type:        "Point",
        Coordinates: []float64{121.6, 31.3},
    },
    Attributes: map[string]interface{}{
        "population": 26400000,
    },
})

// 删除
err = pointDS.Delete(2)
```

### 线数据集

```go
lineDS, err := ds.GetLineDataset("roads")

// 插入线要素
lineFeature := &udbx4go.Feature{
    ID: 1,
    Geometry: &udbx4go.MultiLineStringGeometry{
        Type: "MultiLineString",
        Coordinates: [][][]float64{
            {{116.4, 39.9}, {116.5, 39.8}, {116.6, 39.85}},
        },
    },
    Attributes: map[string]interface{}{
        "name":   "高速公路 1",
        "length": 15.5,
    },
}
err = lineDS.Insert(lineFeature)
```

### 面数据集

```go
regionDS, err := ds.GetRegionDataset("districts")

// 插入多边形要素
regionFeature := &udbx4go.Feature{
    ID: 1,
    Geometry: &udbx4go.MultiPolygonGeometry{
        Type: "MultiPolygon",
        Coordinates: [][][][]float64{
            {
                {{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}},
            },
        },
    },
    Attributes: map[string]interface{}{
        "name": "区域 A",
        "area": 100.0,
    },
}
err = regionDS.Insert(regionFeature)
```

### 属性表数据集

```go
tabularDS, err := ds.GetTabularDataset("attributes")

// 插入记录
record := &udbx4go.TabularRecord{
    ID: 1,
    Attributes: map[string]interface{}{
        "code":  "ATTR001",
        "value": 99.9,
    },
}
err = tabularDS.Insert(record)

// 更新
err = tabularDS.Update(1, map[string]interface{}{
    "value": 100.0,
})
```

## 公开 API 稳定语义

`udbx4go` 的公开 API 遵循 `udbx4spec/docs/08-api-stable-surface.md`。Go 侧使用同步方法和 `(value, error)` 返回形式，但跨语言语义必须与 `udbx4j`、`udbx4ts` 一致。

| 语义 | Go API | 稳定行为 |
|---|---|---|
| 打开数据源 | `Open(path)` | 打开已有 UDBX 文件 |
| 创建数据源 | `Create(path)` | 创建 UDBX 并初始化系统表 |
| 列出数据集 | `ListDatasets()` | 返回 `DatasetInfo` 列表 |
| 按名称获取数据集 | `GetDataset(name)` / 类型专用 getter | 不存在时返回 not found error |
| 按 ID 获取对象 | `GetByID(id)` | 不存在时返回 `nil, err`，且 `udbx4go.IsNotFound(err)` 为 `true` |
| 列表读取 | `List(options)` | 默认按 `SmID` 升序返回 |
| 计数 | `Count()` | 读取物理表真实行数，不以 `SmRegister.SmObjectCount` 缓存为准 |
| 写入 | `Insert(...)` / `InsertMany(...)` | 写入对象并同步对象数 |
| 更新 | `Update(id, ...)` | 目标不存在时返回 not found；未知字段返回 not found 或约束类错误 |
| 删除 | `Delete(id)` | 目标不存在时返回 not found；成功后同步对象数 |

错误处理示例：

```go
feature, err := pointDS.GetByID(42)
if err != nil {
    if udbx4go.IsNotFound(err) {
        // 数据集、字段、记录或要素不存在
        return
    }
    log.Fatal(err)
}
log.Println(feature.ID)
```

## 数据集类型

| 数据集类型 | 描述 | 几何类型 |
|------------|------|----------|
| `Tabular` | 纯属性表 | 无 |
| `Point` | 二维点数据集 | 点 |
| `Line` | 二维线数据集 | 多线串 |
| `Region` | 二维面数据集 | 多多边形 |
| `PointZ` | 三维点数据集 | 点（含 Z） |
| `LineZ` | 三维线数据集 | 多线串（含 Z） |
| `RegionZ` | 三维面数据集 | 多多边形（含 Z） |
| `Text` | 文本标注数据集 | GeoText 最小基线 |
| `CAD` | CAD 数据集 | 自定义 GeoHeader（GeoPoint / GeoLine / GeoRegion） |

## 字段类型

| 字段类型 | Go 类型 | SQLite 类型 |
|----------|---------|-------------|
| `Boolean` | `bool` | INTEGER |
| `Byte` | `int8` | INTEGER |
| `Int16` | `int16` | INTEGER |
| `Int32` | `int32` | INTEGER |
| `Int64` | `int64` | INTEGER |
| `Single` | `float32` | REAL |
| `Double` | `float64` | REAL |
| `Date` | `string` | TEXT |
| `Time` | `string` | TEXT |
| `Char` | `string` | TEXT |
| `Text` | `string` | TEXT |
| `NText` | `string` | TEXT |
| `Binary` | `[]byte` | BLOB |
| `Geometry` | `[]byte` | BLOB |

## 错误处理

udbx4go 提供了特定的错误类型来处理不同的失败场景：

```go
dataset, err := ds.GetDataset("nonexistent")
if err != nil {
    if errors.Is(err, udbx4go.ErrNotFound) {
        // 处理不存在的情况
    } else if udbxErr, ok := err.(udbx4go.UdbxError); ok {
        log.Printf("UDBX 错误 [%s]: %v", udbxErr.Code(), err)
    }
}
```

### 错误检查函数

| 函数 | 说明 |
|------|------|
| `IsFormatError(err)` | 无效的 UDBX 格式 |
| `IsNotFound(err)` | 数据集或要素不存在 |
| `IsUnsupported(err)` | 不支持的操作 |
| `IsConstraintViolation(err)` | 数据约束冲突 |
| `IsIOError(err)` | 文件 I/O 错误 |

## 查询选项

```go
// 获取前 10 个要素
opts := &udbx4go.QueryOptions{Limit: 10}
features, err := dataset.List(opts)

// 获取指定 ID 的要素
opts := &udbx4go.QueryOptions{IDs: []int{1, 3, 5}}
features, err := dataset.List(opts)

// 分页（跳过前 20 个，获取后 10 个）
opts := &udbx4go.QueryOptions{Limit: 10, Offset: 20}
features, err := dataset.List(opts)
```

## 规范

本库遵循 [udbx4spec](https://github.com/udbx4x/udbx4spec) 跨语言规范，与以下实现兼容：

- [udbx4j](https://github.com/udbx4x/udbx4j) - Java 实现
- [udbx4ts](https://github.com/udbx4x/udbx4ts) - TypeScript 实现

## 开发

### 前置要求

- Go 1.21 或更高版本
- C 编译器（用于 SQLite CGO 绑定）

### 设置

```bash
# 克隆仓库
git clone https://github.com/udbx4x/udbx4go.git
cd udbx4go

# 安装依赖
go mod download
```

### 测试

```bash
# 运行所有测试
go test ./...

# 运行并生成覆盖率报告
go test -cover ./...

# 生成 HTML 覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### 代码质量

```bash
# 格式化代码
go fmt ./...

# 运行静态检查
go vet ./...

# 使用 race detector 运行测试
go test -race ./...
```

## 项目结构

```
udbx4go/
├── pkg/                    # 公共 API
│   ├── types/              # 核心类型（DatasetKind、FieldType、Geometry 等）
│   └── errors/             # 错误类型和处理
├── internal/               # 内部实现
│   ├── codec/              # 二进制编解码器（GAIA、CAD）
│   ├── dataset/            # 数据集实现（点、线、面、属性表）
│   ├── schema/             # 模式初始化
│   └── system/             # 系统表 DAO
├── cmd/                    # 示例应用程序
├── udbx.go                 # 主包文档和重导出
├── datasource.go           # DataSource 实现
└── README.md
```

## 架构

### 核心设计原则

- **不可变数据**：优先创建新对象，而非修改现有对象
- **基于接口的设计**：小而专注的接口，便于扩展
- **错误包装**：所有错误都包含上下文信息，便于调试
- **TDD 方法**：先写测试，再写实现

### 二进制格式

UDBX 使用 SQLite 作为底层存储，采用 GAIA 几何编码：

```
GAIA 点头部（43 字节）：
0x00 | 字节序(0x01) | srid(int32) | MBR(4×double) | 0x7c | geoType(int32)
```

### 系统表

| 表 | 用途 |
|----|------|
| `SmRegister` | 数据集元数据（名称、类型、边界、数量） |
| `SmFieldInfo` | 字段元数据（名称、类型、别名、可空） |
| `geometry_columns` | 几何列注册 |
| `SmDataSourceInfo` | 文件级元数据 |

## 贡献

欢迎贡献！请确保：

1. 所有测试通过（`go test ./...`）
2. 保持代码覆盖率（当前 76%+）
3. 遵循 Go 最佳实践（`go fmt`、`go vet`）
4. 使用 race detector 运行测试（`go test -race ./...`）
5. 为新功能添加测试
6. 根据需要更新文档
7. 公开发布前按工作区根目录 `RELEASE.md` 执行完整发布门禁

## 许可证

MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## 相关项目

- [udbx4spec](https://github.com/udbx4x/udbx4spec) - 跨语言规范
- [udbx4j](https://github.com/udbx4x/udbx4j) - Java 实现
- [udbx4ts](https://github.com/udbx4x/udbx4ts) - TypeScript 实现
