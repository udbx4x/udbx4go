# udbx4go-viewer

`udbx4go-viewer` 是基于 `udbx4go` 的 UDBX 桌面查看器，当前实现使用 Wails v2 + React + TypeScript。

## 定位

本工具属于 `udbx4x` 工具生态，必须依赖 `udbx4go` SDK，不得复制 UDBX 底层解析逻辑。

当前目标：

- 打开本地 `.udbx` 文件。
- 列出文件中的数据集。
- 按页查看数据记录。
- 使用轻量地图工作台布局：左侧数据集、中央地图、右侧检查器、底部属性表抽屉。
- 创建一副只读地图，同时预览多个空间数据集图层。
- 支持地图图层显隐、通过更多菜单移除图层，以及适配全部可见图层。
- 支持属性表折叠、半展开和全展开，默认不会长期压缩地图预览区域。
- 右侧检查器提供图层、属性、样式三个视图，并为后续图层样式设置保留入口。
- 左侧数据集浏览器支持名称搜索、按类型过滤，并显示轻量“已加入”状态。
- 采样预览图层会在图层面板和地图区域显示轻量提示。
- 点、线、面图层使用内置默认样式渲染，并在内部保留 `LayerStyle` 结构供后续样式设置 UI 使用。
- 在地图、图层和属性表之间按 `datasetName + SmID` 做单选联动。
- 点击地图要素或属性表行查看属性摘要。
- 支持本机设置模块，配置空间预览上限、地图定位行为、属性表默认状态和预览统计显示。
- 显示加载状态和错误信息。
- 关闭或切换文件时释放当前 `DataSource`。

当前不提供：

- 在线底图、投影变换或坐标转换。
- 编辑或保存 UDBX 文件。
- 空间数据编辑、保存、导出或格式转换。
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
- `GetDatasetSpatialSummary(datasetName)`：读取空间预览摘要。
- `LoadSpatialPreview(datasetName, request)`：读取有限数量空间预览要素。
- `GetFeatureAttributes(datasetName, featureID)`：按 `SmID` 查询要素属性摘要。
- `GetViewerSettings()`：读取 viewer 本机设置。
- `SaveViewerSettings(settings)`：保存 viewer 本机设置。
- `ResetViewerSettings()`：恢复默认 viewer 设置。

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

前端测试与构建：

```bash
cd udbx4go/cmd/udbx4go-viewer/frontend
npm test
npm run build
```

当前前端组件测试覆盖顶部工具栏、数据集浏览器、图层面板、要素属性面板、App 状态装配和测试夹具基础。UI 布局和跨组件联动改动还应按以下手工脚本验收：

```text
1. 启动 wails dev。
2. 初始状态下顶部显示“未打开文件”，更多文件操作不可用。
3. 打开 data/SampleData.udbx 后，左侧显示数据集列表，顶部显示 SampleData.udbx。
4. 选择 BaseMap_P 后，中央地图显示点图层，右侧图层列表显示 BaseMap_P。
5. 再选择 BaseMap_L 和 BaseMap_R 后，中央地图同时显示点、线、面图层，右侧图层列表显示三个图层。
6. 在右侧图层列表关闭 BaseMap_P 可见性后，点图层隐藏，线和面仍显示。
7. 从右侧图层列表的更多菜单移除 BaseMap_L 后，线图层从地图消失，删除按钮不常驻暴露。
8. 右侧检查器可在“图层”“属性”“样式”之间切换。
9. 点击地图要素后，右侧属性视图显示 datasetName、SmID、几何类型和属性摘要。
10. 底部属性表支持折叠、半展开和全展开；默认半展开，不会长期压缩地图预览区域。
11. 属性表显示当前数据集记录；点击属性表行后，地图高亮同一 datasetName + SmID 的要素。
12. 收起底部属性表后，地图获得更大预览空间。
13. 选择 TabularDT 后，属性表切换为 TabularDT，地图已有空间图层不被清空。
14. 打开不存在或损坏文件时显示错误提示，已打开文件状态不被错误数据污染。
15. 打开设置，修改空间预览顶点预算并保存，重新打开设置后值保持。
16. 关闭“加载图层后自动适配范围”后新增图层，地图不自动 fit；重新开启后恢复自动 fit。
17. 开启“显示空间预览统计”后，右侧图层面板显示预览要素数、顶点数和采样状态。
18. 使用 henan.udbx 的县级行政区划数据验证 164 条面要素可完整预览，并且属性表第 2 页记录点击后可在地图上高亮和定位。
19. 长数据集名称不被“已加入”状态覆盖，hover 可看到完整名称。
20. 左侧数据集可按名称搜索，并可按全部、空间、表格、未知过滤。
21. 属性表支持折叠、半展开和全展开，默认不会长期压缩地图。
22. 右侧检查器可在图层、属性、样式之间切换。
23. 图层移除操作位于更多菜单中，不常驻暴露删除按钮。
24. 采样预览图层在图层面板和地图区域都有轻量提示。
```

后续稳定化任务应继续补充前端测试覆盖，重点覆盖加载状态、错误提示、数据集选择、分页交互和地图联动。

## macOS 本机性能与打包验收

该流程只用于当前 macOS 电脑上的可重复验收，不是 CI 门禁，也不把 `henan.udbx`、原始结果或构建产物提交到仓库。运行前需要安装 Go、Wails v2、Node.js、npm 和 `jq`；脚本还会使用 macOS 自带的 `ps`、`awk`、`shasum`、`stat`、`sw_vers` 和 `sysctl`。

完整执行命令：

```bash
cd udbx4go
./scripts/run-viewer-macos-benchmark.sh \
  --sample-data /absolute/path/to/SampleData.udbx \
  --henan-data /absolute/path/to/henan.udbx \
  --output-dir "$PWD/.benchmark-results/manual-run"
```

未传入样本路径时，脚本默认读取工作区 `data/SampleData.udbx` 和 `data/henan.udbx`；未传入输出目录时，结果写入 `.benchmark-results/<timestamp>/`。脚本默认执行 `wails build -platform darwin/universal -skipbindings`。已经确认当前 `.app` 是待测版本时，可以传入 `--skip-build` 复用现有构建。

固定场景：

- `sampledata-multilayer`：打开 `SampleData.udbx`，加载 `BaseMap_P`、`BaseMap_L`、`BaseMap_R` 和 `CADDT`，适配全部图层，并选择 `BaseMap_R` 第 1 页第 1 条记录。
- `henan-county-page-2`：打开 `henan.udbx`，加载“县级行政区划”，适配图层，并选择第 2 页第 1 条记录。

每个场景启动打包应用 5 次，共产生 10 轮结果。第 1 轮标记为 `cold`，其余 4 轮标记为 `warm`；每轮都是独立应用进程，“热运行”表示同一轮批次中的后续启动，仍可能受 macOS 文件缓存和 WebKit 缓存影响。报告保留冷运行原值，并计算四次热运行的中位数和最慢值。

耗时指标：

- `openFileMs`：打开 UDBX 文件并读取数据集列表。
- `loadLayersMs`：加载场景要求的空间图层。
- `fitVisibleLayersMs`：适配全部可见图层范围。
- `selectAndFitMs`：读取指定分页记录、查询属性、高亮并定位要素。

`peakRssKiB` 由外部脚本每 100 ms 对 Viewer 根进程及其全部后代进程的 RSS 求和并取最大值，因此包含 Wails 应用和 WebKit 子进程。没有采集到有效 RSS 时，结果通过 `memoryCaptureError` 明确记录，不能把 `0` 当作真实内存值。

结果目录结构：

```text
.benchmark-results/<run>/
├── configs/       # 每轮只读基准配置
├── raw/           # 10 份带环境和 RSS 的原始 JSON
├── *.log          # 每轮打包应用日志
├── summary.json   # 机器可读汇总
└── summary.md     # 性能表、失败详情和人工验收清单
```

缺少样本、构建失败、应用超时、结果不完整或任一轮 `status != passed` 时，脚本以非零状态退出；已经生成的原始结果和失败原因仍会保留。自动基准通过后，还必须使用同一次构建的 `.app` 完成人工验收：

1. `SampleData.udbx` 的点、线、面和 `CADDT` 可同时显示，图层显隐和移除正常。
2. 地图与属性表双向选择正常，点、线、面按各自几何范围定位。
3. `henan.udbx` 的县级行政区划完整显示 164 条，第 2 页记录可高亮定位。
4. Viewer 设置修改后可持久化，采样状态和错误提示可见。
5. 损坏文件或不支持数据集显示错误，应用不白屏、不崩溃。

人工结果填写到本次 `summary.md`。性能数据只适合作为当前电脑、当前系统和当前样本的本机基线；不同 CPU、内存、macOS 版本、后台负载或文件缓存条件下的绝对数值不能直接比较，也不能据此判定性能回归。

## 维护约束

- 数据读取必须通过 `udbx4go` SDK。
- 打开新文件前必须释放旧 `DataSource`。
- 分页读取必须限制每页数量。
- 前端仅负责展示和交互，不直接解析 UDBX。
- 若 viewer 需要新的 UDBX 能力，应先在 `udbx4go` 中实现。

## 历史方案状态

早期与当前实现冲突的 viewer 方案文档已删除。后续维护以当前 Wails v2 实现为准。
