# udbx-validator

`udbx-validator` 是只读 UDBX 合规验证 CLI，基于 `udbx4go` SDK 实现。

## 用法

```bash
go run ./cmd/udbx-validator --format markdown ../data/SampleData.udbx
go run ./cmd/udbx-validator --format json ../data/SampleData.udbx
```

## Exit Code

- `0`：完成验证且没有 fail。
- `1`：完成验证但存在 validation fail。
- `2`：CLI 参数错误、文件不可读、无法打开文件或运行环境错误。

## 边界

- 只读，不修改输入文件。
- 不复制 SQLite 系统表解析、GAIA、GeoText、CAD 或字段映射逻辑。
- SDK 缺失的诊断能力先补 `udbx4go` 公开 API。
