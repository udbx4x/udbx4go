# udbx4go

[![Go Reference](https://pkg.go.dev/badge/github.com/udbx4x/udbx4go.svg)](https://pkg.go.dev/github.com/udbx4x/udbx4go)
[![Go Report Card](https://goreportcard.com/badge/github.com/udbx4x/udbx4go)](https://goreportcard.com/report/github.com/udbx4x/udbx4go)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](./)
[![Coverage](https://img.shields.io/badge/coverage-76.7%25-yellowgreen)](./)

UDBX（通用空间数据库扩展）读写库的 Go 语言实现。UDBX 是一种基于 SQLite 的空间数据格式，支持矢量（点、线、面、CAD）和属性表数据集。

[English](./README.md)

## 特性

- ✅ 完整的 UDBX 格式支持（读/写）
- ✅ 全部数据集类型：点、线、面、三维点、三维线、三维面、属性表、文本、CAD
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
changes := &udbx4go.FeatureChanges{
    Attributes: map[string]interface{}{
        "population": 27000000,
    },
}
err = pointDS.Update(2, changes)

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
| `Text` | 文本标注数据集 | 文本 |
| `CAD` | CAD 数据集 | 自定义 |

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

## 许可证

MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## 相关项目

- [udbx4spec](https://github.com/udbx4x/udbx4spec) - 跨语言规范
- [udbx4j](https://github.com/udbx4x/udbx4j) - Java 实现
- [udbx4ts](https://github.com/udbx4x/udbx4ts) - TypeScript 实现
