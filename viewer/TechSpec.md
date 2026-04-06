# udbx4go-viewer 技术方案

## 1. 项目概述

udbx4go-viewer 是一个基于 Go 和 Fyne 的图形化工具，用于直观查看和验证 UDBX 文件内容。

## 2. 技术选型

### 2.1 GUI 框架：Fyne

```
fyne.io/fyne/v2 v2.5.0
```

选择理由：
- 纯 Go 实现，无 CGO 依赖
- 跨平台（Windows/macOS/Linux）
- 提供所需全部组件
- API 简洁，开发效率高

### 2.2 项目结构

```
cmd/udbx4go-viewer/
├── main.go              # 程序入口
├── app.go               # 主应用窗口
├── tree.go              # 数据集树形控件
├── table.go             # 数据表格
├── icons.go             # 数据集类型图标
├── dataset.go           # 数据集加载逻辑
└── go.mod
```

## 3. 核心功能设计

### 3.1 文件打开
- 使用 `dialog.NewFileOpen()` 选择 .udbx 文件
- 调用 `udbx4go.Open()` 打开
- 错误处理：显示对话框提示

### 3.2 数据集列表
- 使用 `widget.Tree` 展示
- 每个节点显示：图标 + 数据集名称 + 记录数
- 根据 `DatasetKind` 显示不同图标和颜色

### 3.3 数据表格
- 使用 `widget.Table` 展示
- 动态列：SmID + Geometry(可选) + 属性字段
- 分页：每页 100 条，底部显示页码导航

### 3.4 支持的数据集类型

| 类型 | 图标颜色 | 说明 |
|------|---------|------|
| Tabular | 灰色 | 纯属性表 |
| Point | 蓝色 | 2D 点 |
| PointZ | 深蓝色 | 3D 点 |
| Line | 绿色 | 2D 线 |
| LineZ | 深绿色 | 3D 线 |
| Region | 橙色 | 2D 面 |
| RegionZ | 深橙色 | 3D 面 |
| CAD | 紫色 | CAD 数据集 |

## 4. 数据加载策略

### 4.1 懒加载
- 打开文件时只加载数据集元数据
- 点击数据集时才加载实际数据

### 4.2 分页加载
```go
opts := &types.QueryOptions{
    Limit:  pageSize,    // 100
    Offset: (page-1) * pageSize,
}
features, err := dataset.List(opts)
```

### 4.3 并发处理
- 数据加载在 goroutine 中执行
- 使用 channel 通知 UI 更新
- 避免阻塞主线程

## 5. 界面布局

```
+--------------------------------------------------+
| 文件  帮助                                        |
+-----------+--------------------------------------+
|           |                                      |
| 数据集    |  SmID | Geometry    | 字段1 | 字段2  |
|           |  -----+-------------+-------+-------+
| [图标]    |   1   | POINT(...)  |  val  |  val  |
| Dataset1  |   2   | POINT(...)  |  val  |  val  |
| (5条)     |  ...  |    ...      |  ...  |  ...  |
|           |                                      |
| [图标]    |  [<<] [<] 第 1/5 页 [>] [>>]        |
| Dataset2  |                                      |
| (12条)    +--------------------------------------+
|           | 状态: 已打开 SampleData.udbx         |
+-----------+--------------------------------------+
```

## 6. 错误处理

| 场景 | 处理方式 |
|------|---------|
| 文件打开失败 | 对话框显示错误信息 |
| 数据集加载失败 | 状态栏提示 + 日志 |
| 数据解析失败 | 表格显示 (error) |
| 空数据集 | 显示 "无数据" 提示 |

## 7. 资源管理

- 窗口关闭时调用 `DataSource.Close()`
- 切换文件时先关闭旧连接
- 使用 `defer` 确保资源释放

## 8. 构建与运行

```bash
cd cmd/udbx4go-viewer
go mod tidy
go build -o udbx4go-viewer
./udbx4go-viewer
```

## 9. 依赖清单

```go
require (
    fyne.io/fyne/v2 v2.5.0
    github.com/udbx4x/udbx4go v0.1.0
)
```
