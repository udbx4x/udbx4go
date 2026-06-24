# udbx4go-viewer

`udbx4go-viewer` 是基于 `udbx4go` 的 UDBX 桌面查看器，当前实现使用 Wails v2 + React + TypeScript。

## 定位

本工具属于 `udbx4x` 工具生态，必须依赖 `udbx4go` SDK，不得复制 UDBX 底层解析逻辑。

当前目标：

- 打开本地 `.udbx` 文件。
- 列出文件中的数据集。
- 按页查看数据记录。
- 显示加载状态和错误信息。

## 技术栈

- Go 后端。
- Wails v2 桌面壳。
- React + TypeScript 前端。
- `udbx4go` 作为 UDBX 读取能力来源。

## 目录结构

```text
cmd/udbx4go-viewer/
├── app.go
├── main.go
├── wails.json
├── frontend
│   ├── src
│   └── package.json
└── build
```

## 开发运行

```bash
cd udbx4go/cmd/udbx4go-viewer
wails dev
```

## 构建

```bash
cd udbx4go/cmd/udbx4go-viewer
wails build
```

## 后端能力

当前 Go 后端提供：

- `OpenFileDialog()`：打开系统文件选择对话框。
- `OpenUDBXFile(path)`：打开 UDBX 文件并读取文件信息。
- `CloseUDBXFile()`：关闭当前数据源。
- `ListDatasets()`：列出数据集。
- `GetDatasetFields(datasetName)`：读取字段列表。
- `LoadDatasetPage(datasetName, page)`：分页读取数据。

## 维护约束

- 数据读取必须通过 `udbx4go` SDK。
- 打开新文件前必须释放旧 `DataSource`。
- 分页读取必须限制每页数量。
- 前端仅负责展示和交互，不直接解析 UDBX。
- 若 viewer 需要新的 UDBX 能力，应先在 `udbx4go` 中实现。

## 历史方案状态

早期与当前实现冲突的 viewer 方案文档已删除。后续维护以当前 Wails v2 实现为准。
