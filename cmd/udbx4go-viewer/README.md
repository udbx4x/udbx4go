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
- Point、Line、Region 及对应 Z 类型按 OpenLayers 稳定视口查询；首次加入图层时先按有效的数据集声明范围定位，再由 `moveend` 触发当前范围加载。
- SDK 视口查询支持 RTree 和 DataSource 生命周期包络缓存，Text/CAD 正常路径与普通矢量共用 `QuerySpatial`、required ID 和视口协调器；Viewer 仅在缓存预算超限或空间索引不可用时生成私有的非空间有界预览，并显示当前范围、截断和降级状态。
- 点、线、面图层使用内置默认样式渲染，并在内部保留 `LayerStyle` 结构供后续样式设置 UI 使用。
- 在地图、图层和属性表之间按 `datasetName + SmID` 做单选联动。
- 点击地图要素或属性表行查看属性摘要。
- 支持本机设置模块，配置空间预览上限、地图定位行为、属性表默认状态和预览统计显示。
- 显示加载状态和错误信息。
- 关闭或切换文件时释放当前 `DataSource`。

当前不提供：

- 在线底图、投影变换或坐标转换。
- 坐标投影转换和精确拓扑谓词；当前视口查询语义仍是对象 MBR 与视口 MBR 相交。
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
- 视口参数传入 SDK，Point/Line/Region/Z/Text/CAD 的 `rtree | envelope_cache` 成功策略、`hasMore`、required ID 和声明范围正确映射；仅缓存预算超限或空间索引不可用时由 Viewer 转换为带降级原因的私有预览 DTO。
- 非法视口、context 取消、损坏几何、迟到结果和文件生命周期隔离。

前端测试与构建：

```bash
cd udbx4go/cmd/udbx4go-viewer/frontend
npm test
npm run build
```

当前前端测试覆盖稳定 `moveend`、100 ms 防抖、每图层一个执行中请求加一个最新待执行请求、全局并发、迟到结果丢弃、Source 原子替换、多图层状态、视口外选择、确定性陈旧结果探针和 canvas 像素门禁。实现与非 GUI 自动门禁已完成；真实打包运行和交互证据仍应按以下脚本执行：

```text
1. 启动 wails dev。
2. 初始状态下顶部显示“未打开文件”，更多文件操作不可用。
3. 打开 data/SampleData.udbx 后，左侧显示数据集列表，顶部显示 SampleData.udbx。
4. 选择 BaseMap_P 后，地图先按数据集声明范围定位，再加载当前视口点要素；右侧图层列表显示 BaseMap_P。
5. 再选择 BaseMap_L 和 BaseMap_R 后，中央地图同时显示点、线、面图层，右侧图层列表显示三个图层。
6. 在右侧图层列表关闭 BaseMap_P 可见性后，点图层隐藏，线和面仍显示。
7. 从右侧图层列表的更多菜单移除 BaseMap_L 后，线图层从地图消失，删除按钮不常驻暴露。
8. 右侧检查器可在“图层”“属性”“样式”之间切换。
9. 点击地图要素后，右侧属性视图显示 datasetName、SmID、几何类型和属性摘要。
10. 底部属性表支持折叠、半展开和全展开；默认半展开，不会长期压缩地图预览区域。
11. 属性表显示当前数据集记录；点击视口外记录后，地图按该对象 BBox 定位，并通过 required ID 在新范围中保留和高亮同一 datasetName + SmID 的要素。
12. 收起底部属性表后，地图获得更大预览空间。
13. 选择 TabularDT 后，属性表切换为 TabularDT，地图已有空间图层不被清空。
14. 打开不存在或损坏文件时显示错误提示，已打开文件状态不被错误数据污染。
15. 打开设置，修改空间预览顶点预算并保存，重新打开设置后值保持。
16. 关闭“加载图层后自动适配范围”后新增图层，地图不自动 fit；重新开启后恢复自动 fit。
17. 开启“显示空间预览统计”后，右侧图层面板显示查询策略、耗时、当前范围要素数、顶点数、截断和降级状态。
18. 使用 henan.udbx 的 weibo 数据验证平移缩放按当前范围加载；放大到结果未截断时，“当前范围 1,000+ 个对象”提示消失。
19. 使用 henan.udbx 的县级行政区划验证无 RTree 时走包络缓存；属性表第 2 页记录点击后可定位并高亮。
20. 使用 SampleData.udbx 验证点、线、面、Text、CAD 按视口更新；图层显隐和移除正常。
21. 快速连续缩放时不白屏、不回跳到旧范围；单图层查询失败时保留旧图形，其他图层仍可用。
22. 关闭或切换文件后，旧请求结果不写入新地图。
```

### 设置与诊断

- “空间预览要素上限”默认 1,000，限制一次视口查询的普通候选数。存在更多视口命中对象时显示当前范围截断提示；required ID 不占用该上限。
- SDK 返回的普通要素必须与请求范围 MBR 相交；只有通过 required ID 补入的选中对象可以位于视口外。
- “空间预览顶点预算”默认 1,000,000，限制一次响应发送和渲染的普通几何顶点总量，用于控制 Wails 传输和前端渲染成本。它不是数据集对象数或 UDBX 格式限制。
- 未提供 viewport 时，后端使用完整的有限 `float64` 坐标域调用公共 `QuerySpatial`，`QueriedBounds` 保持为空；声明 extent 只用于自动定位提示，不作为完整性边界。采样状态由实际 `HasMore` 和顶点预算截断决定，不依赖 `SmObjectCount`。
- 高级预览统计中的 `rtree` 和 `envelope_cache` 是 SDK 成功策略。`bounded_sample` 仅是 Viewer 捕获 `envelope_cache_budget_exceeded` 或 `spatial_index_unavailable` 后生成的私有非空间有界预览，不属于 SDK `QuerySpatial` 成功结果；正常 Text/CAD 不进入该路径。
- SDK `SpatialQueryResult` 没有 `DegradedReason`；Viewer `SpatialPreviewDTO` 保留 `degradedReason` 供界面诊断。
- 当前包络缓存默认策略是单数据集 32 MiB、当前 `DataSource` 合计 64 MiB；关闭或切换文件会释放缓存。完整缓存超预算时 SDK 返回 `envelope_cache_budget_exceeded` 错误。这是当前 SDK/Viewer 资源策略，不是 UDBX 格式限制。
- `invalid_viewport`、`spatial_index_unavailable`、`envelope_cache_budget_exceeded`、`query_timeout`、`corrupt_geometry`、`unsupported_dataset_kind` 是稳定诊断原因码。

无索引包络缓存 PoC 脚本会执行五个规模各 20 次的隔离测量，`--report` 必须使用绝对路径。

从 `udbx4x` 工作区根目录运行时：

```bash
cd udbx4go
./scripts/run-envelope-cache-poc.sh \
  --report "$(cd .. && pwd)/docs/superpowers/reports/2026-07-16-udbx-viewer-envelope-cache-poc.md"
```

从独立 `udbx4go` 仓库根目录运行时，不执行 `cd udbx4go`，并把报告写到用户选择的绝对路径：

```bash
./scripts/run-envelope-cache-poc.sh \
  --report /absolute/path/to/envelope-cache-poc.md
```

项目级 PoC 报告只存在于 `udbx4x` 工作区的 `docs/superpowers/reports/`，不是独立 `udbx4go` 仓库内的相对文件。

## macOS 本机性能与打包验收

该流程只用于当前 macOS 电脑上的可重复验收，不是 CI 门禁，也不把 `henan.udbx`、原始结果或构建产物提交到仓库。运行前需要安装 Go、Wails v2、Node.js、npm 和 `jq`；脚本还会使用 macOS 自带的 `ps`、`awk`、`shasum`、`stat`、`sw_vers` 和 `sysctl`。

从 `udbx4x` 工作区根目录运行完整验收，并将成功报告写回项目级报告路径：

```bash
cd udbx4go
./scripts/run-viewer-macos-benchmark.sh \
  --sample-data /absolute/path/to/SampleData.udbx \
  --henan-data /absolute/path/to/henan.udbx \
  --output-dir "$PWD/.benchmark-results/manual-run" \
  --acceptance-report "$(cd .. && pwd)/docs/superpowers/reports/2026-07-16-udbx-viewer-viewport-spatial-query-acceptance.md"
```

从独立 `udbx4go` 仓库根目录运行时，不执行 `cd udbx4go`。独立仓不包含项目级验收报告，`--acceptance-report` 必须指向用户选择的绝对路径：

```bash
./scripts/run-viewer-macos-benchmark.sh \
  --sample-data /absolute/path/to/SampleData.udbx \
  --henan-data /absolute/path/to/henan.udbx \
  --output-dir "$PWD/.benchmark-results/manual-run" \
  --acceptance-report /absolute/path/to/viewport-spatial-query-acceptance.md
```

未传入样本路径时，脚本默认读取 Go 仓库相邻的 `../data/SampleData.udbx` 和 `../data/henan.udbx`；独立仓通常没有这些相邻样本，因此应显式传入绝对样本路径。未传入输出目录时，结果写入 `.benchmark-results/<timestamp>/`。默认流程会以并发 1 构建同一个 universal `.app`，在同一构建和样本上运行并发 1/2/3 候选，选择较小合格值，然后写入最终常量、重新构建并独立重跑。只有显式指定 `--max-concurrent 1|2|3` 的单套件模式才可配合 `--skip-build`。

固定场景：

- `henan-weibo-rtree-pan-zoom`：验证 weibo 真实 RTree 的连续视口平移缩放和截断状态。
- `henan-county-envelope-selection`：验证县级行政区划包络缓存、第二页对象定位、required ID 和高亮。
- `sampledata-multilayer-viewport`：加载 `BaseMap_P`、`BaseMap_L`、`BaseMap_R`、`County_T` 和 `CADDT`，验证普通矢量、Text、CAD 多图层视口查询、显隐和移除；所有视口步骤都要求 `envelope_cache`，任何 `bounded_sample` 都会使 benchmark 失败。

每个场景执行冷 5 轮和热 5 轮，共 10 轮；每套并发包含 30 轮。默认候选阶段依次运行并发 1/2/3，最终重建后再以选中值独立运行 30 轮，候选结果不能代替最终结果。并发 2/3 只有实际观测达到标称并发、所有门禁通过、端到端 P95 至少改善 5% 且峰值 RSS 不超过并发 1 的 110% 时才可入选。没有真实合格数据时最终并发保持 1。

耗时指标：

- `openFileMs`：打开 UDBX 文件并读取数据集列表。
- `loadLayersMs`：加载场景要求的空间图层。
- `fitVisibleLayersMs`：适配全部可见图层范围。
- `selectAndFitMs`：读取指定分页记录、查询属性、高亮并定位要素。

`peakRssKiB` 由外部脚本每 100 ms 对 Viewer 根进程及其全部后代进程的 RSS 求和并取最大值，因此包含 Wails 应用和 WebKit 子进程。没有采集到有效 RSS 时，结果通过 `memoryCaptureError` 明确记录，不能把 `0` 当作真实内存值。

结果目录结构：

```text
.benchmark-results/<run>/
├── candidates/
│   ├── concurrency-1/
│   ├── concurrency-2/
│   └── concurrency-3/
├── selection.json
├── selection.md
└── final/
    ├── configs/
    ├── raw/
    ├── summary.json
    └── summary.md
```

缺少样本、构建失败、应用超时、结果不完整或任一轮 `status != passed` 时，脚本以非零状态退出，并恢复并发策略文件。候选阶段的并发观测、RSS 或延迟门禁失败只会把该候选标记为不合格，并继续完成其他候选；最终重跑门禁失败仍以非零状态退出。成功验收报告只会在最终门禁通过后原子替换。最终证据必须按来源分类：第 1-4 项是同机同样本人工证据，第 5 项是自动化故障注入，第 6 项是真实切换加自动化生命周期证据。

人工证据：

1. `henan.udbx/weibo` 平移缩放按当前范围加载，放大后截断提示消失。
2. 县级行政区划按视口浏览，第二页对象可定位并高亮。
3. `SampleData.udbx` 点、线、面、Text、CAD 多图层按视口刷新，显隐和移除正常。
4. 快速连续缩放无白屏、无旧范围回跳。

自动化故障注入：

5. 单图层查询失败保留旧图形，其他图层继续可用。

真实切换加自动化生命周期：

6. 真实执行关闭或切换文件，并由自动化生命周期测试证明旧请求结果不写入新地图。

2026-07-18 基于提交 `644d64f6688728ec7ea9a6137b82e58fdc6ea3c2`、应用 SHA256 `2057ac3c02e637fd8ced54f0418ddc8dd8ff7b354d36359219cbc290a9ff6b93` 的并发候选与最终 30 轮结果仅保留为历史性能和交互证据。该运行没有确定性制造 stale request，也没有检查 canvas 非透明像素，因此不能证明当前陈旧结果丢弃和非白屏门禁。历史原始数据位于 `.benchmark-results/final-calibration-644d64f/`，项目级报告位于 `udbx4x` 工作区的 `docs/superpowers/reports/2026-07-16-udbx-viewer-viewport-spatial-query-acceptance.md`；该报告不属于独立 `udbx4go` 仓库。性能数据只适合作为当时电脑、系统和样本的本机基线；不同 CPU、内存、macOS 版本、后台负载或文件缓存条件下的绝对数值不能直接比较。

## 维护约束

- 数据读取必须通过 `udbx4go` SDK。
- 打开新文件前必须释放旧 `DataSource`。
- 分页读取必须限制每页数量。
- 前端仅负责展示和交互，不直接解析 UDBX。
- 若 viewer 需要新的 UDBX 能力，应先在 `udbx4go` 中实现。

## 历史方案状态

早期与当前实现冲突的 viewer 方案文档已删除。后续维护以当前 Wails v2 实现为准。
