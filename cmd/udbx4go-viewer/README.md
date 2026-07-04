# udbx4go-viewer

`udbx4go-viewer` 是基于 `udbx4go` 的 UDBX 桌面查看器，当前实现使用 Wails v2 + React + TypeScript。

## 定位

本工具属于 `udbx4x` 工具生态，必须依赖 `udbx4go` SDK，不得复制 UDBX 底层解析逻辑。

当前目标：

- 打开本地 `.udbx` 文件。
- 列出文件中的数据集。
- 按页查看数据记录。
- 显示加载状态和错误信息。
- 关闭或切换文件时释放当前 `DataSource`。

当前不提供：

- 编辑或保存 UDBX 文件。
- 转换、导出或投影变换。
- 绕过 `udbx4go` SDK 的系统表或二进制几何解析。

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

## 验证

后端测试：

```bash
cd udbx4go/cmd/udbx4go-viewer
go test ./...
```

当前后端测试覆盖：

- 打开 `data/SampleData.udbx`。
- 列出 `BaseMap_P`、`County_T`、`CADDT` 等真实样本数据集。
- 分页读取 `BaseMap_P`。
- 关闭文件后拒绝继续读取。
- 打开不存在文件时清理旧数据源状态。
- 页码小于 1 或大于总页数时归一到有效范围。

前端当前尚未配置自动化测试脚本，`frontend/package.json` 没有 `test` 命令。前端改动至少按以下手工脚本验收：

```text
1. 启动 wails dev。
2. 无文件状态下，“关闭文件”按钮不可用。
3. 打开 data/SampleData.udbx 后，左侧显示数据集列表。
4. 选择 BaseMap_P 后，右侧表格显示第一页记录。
5. 翻页按钮不会触发越界页。
6. 打开不存在或损坏文件时显示错误提示。
7. 关闭文件后状态栏清空当前文件，数据集列表和表格不再显示旧数据。
```

后续稳定化任务应补充前端测试框架，覆盖加载状态、错误提示、数据集选择和分页交互。

## 维护约束

- 数据读取必须通过 `udbx4go` SDK。
- 打开新文件前必须释放旧 `DataSource`。
- 分页读取必须限制每页数量。
- 前端仅负责展示和交互，不直接解析 UDBX。
- 若 viewer 需要新的 UDBX 能力，应先在 `udbx4go` 中实现。

## 历史方案状态

早期与当前实现冲突的 viewer 方案文档已删除。后续维护以当前 Wails v2 实现为准。
