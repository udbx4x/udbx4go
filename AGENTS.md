# AGENTS.md

本文件是 `udbx4go` 子项目的智能体执行入口。根目录 `../AGENTS.md` 是全工作区总规则；本文件只补充 Go SDK 的项目事实、命令、架构约束和测试约定。

## 项目定位

`udbx4go` 是 UDBX 空间数据格式的 Go SDK，目标是发布到 pkg.go.dev，并作为 CLI、viewer、validator 等工具的 Go 侧基础库。

- Module：`github.com/udbx4x/udbx4go`
- Go 版本：`1.22`
- SQLite 驱动：`github.com/mattn/go-sqlite3`
- 测试库：`github.com/stretchr/testify`
- 规范来源：UDBX 白皮书、`../udbx4spec/docs/`、真实 `.udbx` 样本

任何公开 API、格式语义、枚举值、字段映射或跨语言行为变更，必须先更新 `udbx4spec`，再修改 Go 实现。

## 必读文档

1. `../AGENTS.md`：全工作区原则和决策优先级。
2. `README.md` / `README.zh.md`：用户入口。
3. `API.md` / `API.zh.md`：API 说明。
4. `CHANGELOG.md`：版本变化。
5. `udbx4spec-compliance-report.md`：规范合规状态。
6. `cmd/udbx4go-viewer/README.md`：Wails viewer 说明，若维护 viewer。

## 构建与测试命令

优先使用 Makefile：

```bash
make all
make build
make test
make test-unit
make test-integration
make coverage
make fmt
make lint
make clean
```

直接使用 Go：

```bash
go build ./...
go test ./...
go test -race ./...
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
go fmt ./...
go vet ./...
```

真实样本性能基线：

```bash
make benchmark-real-samples
make benchstat-real-samples BASELINE=bench-old.txt CURRENT=bench-new.txt
```

## 项目结构

```text
udbx4go/
├── cmd
│   ├── udbx4go-example
│   └── udbx4go-viewer
├── internal
│   ├── codec
│   ├── dao
│   ├── dataset
│   ├── geometry
│   ├── schema
│   └── system
├── pkg
│   ├── errors
│   └── types
├── datasource.go
├── udbx.go
├── go.mod
└── README.md
```

核心目录职责：

- `pkg/types`：公开核心类型，如 `DatasetKind`、`FieldType`、`Geometry`、`Feature`、`DatasetInfo`。
- `pkg/errors`：错误类型和错误分类。
- `internal/codec`：GAIA、CAD 等二进制编解码。
- `internal/dataset`：数据集实现。
- `internal/schema`：系统表和数据表初始化。
- `internal/system`：系统表 DAO。
- `datasource.go`：DataSource 实现。
- `udbx.go`：包文档和公开类型重导出。
- `cmd/udbx4go-viewer`：Wails v2 + React/TypeScript viewer。

## 架构约束

- `internal` 包中的实现不得泄漏为公开 API。
- 公开 API 应通过 `udbx.go` 重导出常用类型，保持使用体验简洁。
- DatasetKind、FieldType、错误分类和几何模型必须与 `udbx4spec` 同步。
- 所有二进制解析必须使用 Little-Endian。
- SQLite 连接、事务和 rows 必须明确关闭。
- 批量写入必须使用事务。
- 工具层不得复制 SDK 内部格式解析逻辑。

## DatasetKind 映射

| DatasetKind | 值 | 几何格式 |
|---|---:|---|
| `DatasetKindTabular` | 0 | 无 |
| `DatasetKindPoint` | 1 | GAIAPoint |
| `DatasetKindLine` | 3 | GAIAMultiLineString |
| `DatasetKindRegion` | 5 | GAIAMultiPolygon |
| `DatasetKindText` | 7 | GeoText |
| `DatasetKindPointZ` | 101 | GAIAPointZ |
| `DatasetKindLineZ` | 103 | GAIAMultiLineStringZ |
| `DatasetKindRegionZ` | 105 | GAIAMultiPolygonZ |
| `DatasetKindCAD` | 149 | GeoHeader |

## FieldType

必须支持 14 个规范字段类型：

```text
boolean, byte, int16, int32, int64, single, double, date, binary, geometry, char, ntext, text, time
```

## 错误处理

所有 UDBX 错误应实现 `UdbxError` 接口，并提供错误码。

错误类型：

- `UdbxFormatError`
- `UdbxNotFoundError`
- `UdbxUnsupportedError`
- `UdbxConstraintError`
- `UdbxIOError`

包装错误时必须带上下文：

```go
if err != nil {
    return errors.IOError("failed to query features", err)
}
```

检查具体错误类型：

```go
if udbx4go.IsNotFound(err) {
    // handle not found
}
if udbx4go.IsFormatError(err) {
    // handle format error
}
```

## 二进制格式

GAIA Geometry 使用 Little-Endian：

```text
0x00 | byteOrder(0x01) | srid(int32) | MBR(4*double) | 0x7c | geoType(int32) | coords... | 0xFE
```

GAIA header 长度为 43 bytes。

## 测试策略

- 单元测试文件与实现文件放在同一包，命名为 `xxx_test.go`。
- 使用 table-driven tests 和 `t.Run()`。
- 使用 `testify/assert` 和 `testify/require`。
- 每个测试应独立，文件操作使用 `t.TempDir()`。
- 数据库测试应关闭连接和 rows。
- 共享测试工具位于 `internal/dataset/testutil_test.go`。

示例：

```go
func TestComponent_DoSomething(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    result, err := component.DoSomething()

    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

## API 设计模式

常用公开类型通过 `udbx.go` 重导出：

```go
import "github.com/udbx4x/udbx4go"

ds, err := udbx4go.Open("file.udbx")
feature := &udbx4go.Feature{}
```

数据集层级：

```text
Dataset
├── BaseDataset
│   └── TabularDataset
└── VectorDataset
    ├── PointDataset
    ├── LineDataset
    ├── RegionDataset
    ├── PointZDataset
    ├── LineZDataset
    ├── RegionZDataset
    ├── TextDataset
    └── CadDataset
```

## 当前重点

Go SDK 当前已完成 `udbx4spec` 最小合规基线中的 2D/3D 矢量、Tabular、Text / GeoText 与 CAD 最小 GeoHeader 读写闭环。后续重点是扩大真实 SuperMap UDBX 样本兼容范围、完善性能基准与发布治理。

`TextDataset` 支持最小 GeoText 编解码、`SmIndexKey` 写出、CRUD 与 `CreateTextDataset()` / `GetTextDataset()`。`CadDataset` 已完成 CAD 最小 GeoHeader 基线，覆盖 `CadPointGeometry`、`CadLineGeometry`、`CadRegionGeometry` 的读写和 CRUD，并已纳入 `udbx4spec` 三端 roundtrip 合规资产。

## 任务完成自检

- 是否先确认 `udbx4spec` 是否需要变更。
- 是否补齐 Go 测试和合规测试。
- 是否保持公开 API 简洁。
- 是否关闭数据库资源。
- 是否更新 README、CHANGELOG 或合规报告。
- 是否避免复制其他 SDK 的实现细节而忽略 Go 惯用法。
