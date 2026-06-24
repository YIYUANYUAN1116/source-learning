# kata-bench-analyzer

一个用于练习 Go 的小型 CLI 工具，同时贴近 Kata / QEMU / UnixBench 性能分析场景。

它会读取 benchmark CSV，按照：

- target
- operation
- concurrency

进行分组统计，并输出 Markdown 报告。

---

## 1. 这个项目练什么

这个小工具覆盖了不少 Go 基础能力：

- `go mod` 项目结构
- `flag` 命令行参数
- 文件读取
- CSV 解析
- `struct`
- `slice`
- `map`
- 错误处理 `error`
- `defer`
- 排序 `sort`
- 字符串拼接 `strings.Builder`
- Markdown 报告生成
- 单元测试

后面还可以继续扩展：

- 支持 UnixBench 原始文本解析
- 支持 fio JSON 解析
- 支持多个输入文件对比
- 支持输出 HTML
- 支持生成折线图
- 支持并发执行 benchmark 命令

---

## 2. 项目结构

```text
kata-bench-analyzer/
├── go.mod
├── main.go
├── README.md
├── internal/
│   ├── analyzer/
│   │   ├── stats.go
│   │   └── stats_test.go
│   ├── parser/
│   │   └── csv.go
│   └── report/
│       └── markdown.go
└── testdata/
    └── unixbench.csv
```

说明：

- `main.go`：程序入口，负责读取命令行参数
- `internal/parser`：负责解析 CSV
- `internal/analyzer`：负责统计平均值、最大值、最小值、线性度
- `internal/report`：负责生成 Markdown 报告
- `testdata`：测试数据

---

## 3. 输入 CSV 格式

CSV 头：

```csv
target,concurrency,round,operation,score
```

示例：

```csv
target,concurrency,round,operation,score
container-runc,1,1,System Call Overhead,100000
container-runc,2,1,System Call Overhead,185000
kata-qemu,1,1,System Call Overhead,90000
kata-qemu,2,1,System Call Overhead,150000
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| target | 测试对象，例如 `container-runc`、`kata-qemu`、`vm-qemu` |
| concurrency | 并发数，例如 1、2、4、8 |
| round | 第几轮测试 |
| operation | 测试项，例如 `System Call Overhead` |
| score | 测试分数，越大越好 |

---

## 4. 运行

进入目录：

```bash
cd go/kata-bench-analyzer
```

直接运行：

```bash
go run .
```

指定输入文件：

```bash
go run . -input testdata/unixbench.csv
```

输出到 Markdown 文件：

```bash
go run . -input testdata/unixbench.csv -output report.md
```

指定基准并发数：

```bash
go run . -input testdata/unixbench.csv -baseline 1
```

---

## 5. 输出示例

```markdown
# Kata Bench Analyzer Report

Baseline concurrency: `1`

> Efficiency = current_avg / ideal_linear_avg. 1.00 means perfect linear scaling.

## kata-qemu / System Call Overhead

| concurrency | count | min | max | avg | efficiency |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 2 | 90000.00 | 92000.00 | 91000.00 | 1.00 |
| 2 | 2 | 150000.00 | 152000.00 | 151000.00 | 0.83 |
```

---

## 6. 线性度计算

假设：

```text
1 并发平均分 = 100
2 并发平均分 = 160
```

理想线性增长下：

```text
2 并发理想分数 = 100 * 2 = 200
```

实际线性度：

```text
160 / 200 = 0.8
```

所以 efficiency = `0.8`。

这和你现在分析 Kata + QEMU 线性度瓶颈的思路是一致的。

---

## 7. 测试

运行单元测试：

```bash
go test ./...
```

格式化代码：

```bash
go fmt ./...
```

静态检查：

```bash
go vet ./...
```

---

## 8. 后续可以加的功能

### 8.1 支持原始 UnixBench 输出解析

当前输入是 CSV。后续可以写一个 `parser/unixbench.go`，直接解析 UnixBench 原始输出。

### 8.2 支持 fio JSON

fio 可以输出 JSON：

```bash
fio test.fio --output-format=json --output=fio.json
```

可以新增：

```text
internal/parser/fio_json.go
```

### 8.3 支持多目标对比

例如：

```bash
go run . -input runc.csv,kata.csv,vm.csv
```

然后输出对比报告。

### 8.4 支持执行命令

后续可以加：

```bash
go run . run --target kata --cmd "./Run -c 8"
```

这样就不只是分析数据，还能自动执行测试。

---

## 9. 建议阅读顺序

先看：

```text
main.go
```

然后看：

```text
internal/parser/csv.go
```

再看：

```text
internal/analyzer/stats.go
```

最后看：

```text
internal/report/markdown.go
```

这就是一个典型 Go 小工具的拆分方式。
