# udbx-converter

`udbx-converter` 是最小只读转换器 CLI。当前只支持导出 UDBX 文件清单，不读取 SQLite 系统表、不解析 GAIA、GeoText 或 CAD 二进制内容，只通过 `udbx4go.Open` 和 `ListDatasets` 访问公开 SDK API。

## 用法

```bash
go run ./cmd/udbx-converter inventory --output inventory.json [--overwrite] path/to/file.udbx
```

输出 JSON 结构：

```json
{
  "file": "path/to/file.udbx",
  "datasets": [
    {
      "name": "DatasetName",
      "kind": "point",
      "tableName": "DatasetTable",
      "objectCount": 10
    }
  ]
}
```

## 退出码

- `0`：导出成功。
- `1`：打开 UDBX 或写入 JSON 失败。
- `2`：参数错误、缺少 `--output`，或输出文件已存在但未传 `--overwrite`。

## 写入边界

CLI 不会修改输入 `.udbx` 文件。导出时会创建输出目录，先写入同目录临时文件，再通过 rename 替换目标文件。
