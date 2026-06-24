# Go 基本语法入门

> 目标：先能看懂 Go 代码，再能写小工具，最后再去读 Kata / containerd / Kubernetes 源码。

Go，也叫 Golang，是 Google 开发的一门编程语言，常用于云原生组件、命令行工具、网络服务、并发程序和运维工具。Kubernetes、containerd、Docker、Kata Containers 里都有大量 Go 代码。

---

## 1. 第一个 Go 程序

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, Go")
}
```

说明：

```go
package main
```

表示当前文件属于 `main` 包。`main` 包可以被编译成可执行程序。

```go
import "fmt"
```

导入标准库中的 `fmt` 包，用于格式化输出。

```go
func main()
```

程序入口函数，类似 Java 里的：

```java
public static void main(String[] args)
```

运行：

```bash
go run main.go
```

编译：

```bash
go build -o app main.go
./app
```

---

## 2. 变量

### 2.1 标准声明

```go
var name string = "Aaron"
var age int = 25
```

### 2.2 类型推断

```go
var name = "Aaron"
var age = 25
```

Go 可以根据右边的值自动推断类型。

### 2.3 短变量声明

```go
name := "Aaron"
age := 25
```

这是 Go 里最常见的写法。

注意：`:=` 只能在函数内部使用。

---

## 3. 常量

```go
const Pi = 3.14
const AppName = "kata-bench"
const MaxRetry = 3
```

常量不能被修改。

---

## 4. 基本数据类型

常见类型：

```go
bool
string

int
int8
int16
int32
int64

uint
uint8
uint16
uint32
uint64

float32
float64

byte
rune
```

示例：

```go
var ok bool = true
var name string = "Go"
var age int = 18
var score float64 = 99.5
```

说明：

- `byte` 本质是 `uint8`
- `rune` 本质是 `int32`，常用于表示 Unicode 字符

```go
var c rune = '中'
```

---

## 5. 字符串

```go
s := "hello"
fmt.Println(s)
```

字符串拼接：

```go
name := "Aaron"
msg := "hello " + name
fmt.Println(msg)
```

格式化字符串：

```go
age := 25
msg := fmt.Sprintf("name=%s, age=%d", name, age)
fmt.Println(msg)
```

常见占位符：

```text
%s  字符串
%d  整数
%f  浮点数
%v  默认格式
%+v 结构体详细格式
%#v Go 语法格式
```

示例：

```go
fmt.Printf("name=%s age=%d\n", name, age)
```

---

## 6. 条件判断

```go
age := 18

if age >= 18 {
    fmt.Println("adult")
} else {
    fmt.Println("child")
}
```

Go 的 `if` 条件不需要括号。

### 6.1 if 中声明变量

```go
if age := 20; age >= 18 {
    fmt.Println("adult")
}
```

这个 `age` 变量只在 `if` 语句内部有效。

---

## 7. switch

```go
day := "Mon"

switch day {
case "Mon":
    fmt.Println("Monday")
case "Tue":
    fmt.Println("Tuesday")
default:
    fmt.Println("Other")
}
```

Go 的 `switch` 默认每个 `case` 后自动 break，不需要手写 `break`。

多个条件：

```go
switch day {
case "Sat", "Sun":
    fmt.Println("weekend")
default:
    fmt.Println("workday")
}
```

---

## 8. for 循环

Go 只有 `for`，没有 `while`。

### 8.1 普通 for

```go
for i := 0; i < 5; i++ {
    fmt.Println(i)
}
```

### 8.2 类似 while

```go
i := 0
for i < 5 {
    fmt.Println(i)
    i++
}
```

### 8.3 无限循环

```go
for {
    fmt.Println("running")
}
```

### 8.4 range 遍历

```go
names := []string{"a", "b", "c"}

for index, value := range names {
    fmt.Println(index, value)
}
```

只要值：

```go
for _, value := range names {
    fmt.Println(value)
}
```

只要下标：

```go
for index := range names {
    fmt.Println(index)
}
```

---

## 9. 数组

数组长度固定。

```go
var arr [3]int
arr[0] = 10
arr[1] = 20
arr[2] = 30
```

声明并初始化：

```go
arr := [3]int{10, 20, 30}
```

自动推断长度：

```go
arr := [...]int{10, 20, 30}
```

数组在 Go 中用得不如切片多。

---

## 10. 切片 slice

切片是 Go 中非常常用的数据结构，类似 Java 里的 `ArrayList`。

```go
nums := []int{1, 2, 3}
```

追加元素：

```go
nums = append(nums, 4)
```

遍历：

```go
for i, v := range nums {
    fmt.Println(i, v)
}
```

长度：

```go
fmt.Println(len(nums))
```

切片截取：

```go
nums := []int{1, 2, 3, 4, 5}
a := nums[1:3]
fmt.Println(a)
```

结果：

```text
[2 3]
```

说明：`nums[start:end]` 包含 `start`，不包含 `end`。

---

## 11. map

map 类似 Java 里的 `HashMap`。

```go
m := map[string]int{
    "apple":  3,
    "banana": 5,
}
```

添加或修改：

```go
m["orange"] = 10
```

读取：

```go
count := m["apple"]
fmt.Println(count)
```

判断 key 是否存在：

```go
value, ok := m["apple"]
if ok {
    fmt.Println(value)
} else {
    fmt.Println("not found")
}
```

删除：

```go
delete(m, "apple")
```

遍历：

```go
for k, v := range m {
    fmt.Println(k, v)
}
```

注意：Go 的 map 遍历顺序不是固定的。

---

## 12. 函数

### 12.1 普通函数

```go
func add(a int, b int) int {
    return a + b
}
```

调用：

```go
result := add(1, 2)
fmt.Println(result)
```

### 12.2 参数类型简写

```go
func add(a, b int) int {
    return a + b
}
```

### 12.3 多返回值

Go 支持多个返回值。

```go
func div(a, b int) (int, error) {
    if b == 0 {
        return 0, fmt.Errorf("divide by zero")
    }
    return a / b, nil
}
```

调用：

```go
result, err := div(10, 2)
if err != nil {
    fmt.Println(err)
    return
}
fmt.Println(result)
```

Go 里很常见这种写法：

```go
value, err := doSomething()
if err != nil {
    return err
}
```

---

## 13. 指针

Go 有指针，但没有指针运算。

```go
x := 10
p := &x
fmt.Println(p)
fmt.Println(*p)
```

说明：

- `&x` 获取变量地址
- `*p` 获取指针指向的值

示例：

```go
func update(x *int) {
    *x = 100
}

func main() {
    a := 10
    update(&a)
    fmt.Println(a)
}
```

输出：

```text
100
```

---

## 14. struct 结构体

Go 没有 Java 里的 class，常用 `struct` 表示数据结构。

```go
type User struct {
    Name string
    Age  int
}
```

创建对象：

```go
u := User{
    Name: "Aaron",
    Age:  25,
}
```

访问字段：

```go
fmt.Println(u.Name)
fmt.Println(u.Age)
```

修改字段：

```go
u.Age = 26
```

---

## 15. 方法

Go 的方法是绑定到某个类型上的函数。

```go
type User struct {
    Name string
    Age  int
}

func (u User) SayHello() {
    fmt.Println("hello", u.Name)
}
```

调用：

```go
u := User{Name: "Aaron", Age: 25}
u.SayHello()
```

### 15.1 值接收者

```go
func (u User) SayHello() {
    fmt.Println(u.Name)
}
```

这里的 `u` 是值拷贝。

### 15.2 指针接收者

```go
func (u *User) SetAge(age int) {
    u.Age = age
}
```

如果方法需要修改结构体字段，一般用指针接收者。

---

## 16. interface 接口

Go 的接口是一组方法定义。

```go
type Animal interface {
    Speak() string
}
```

实现接口：

```go
type Dog struct{}

func (d Dog) Speak() string {
    return "wang"
}
```

注意：Go 不需要写 `implements`。

只要一个类型实现了接口中的所有方法，就自动实现了这个接口。

```go
func Say(a Animal) {
    fmt.Println(a.Speak())
}

func main() {
    d := Dog{}
    Say(d)
}
```

这点和 Java 很不一样。

Java 是显式实现：

```java
class Dog implements Animal
```

Go 是隐式实现：

```go
type Dog struct{}
func (d Dog) Speak() string { return "wang" }
```

---

## 17. error 错误处理

Go 不使用 Java 那种 try-catch 异常处理。

Go 通常通过返回 `error` 表示错误。

```go
func readFile(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return err
    }
    defer file.Close()

    return nil
}
```

调用：

```go
err := readFile("a.txt")
if err != nil {
    fmt.Println("read failed:", err)
}
```

包装错误：

```go
return fmt.Errorf("open file failed: %w", err)
```

`%w` 表示把原始错误包进去，方便后续判断。

---

## 18. defer

`defer` 表示函数结束前执行，常用于关闭资源。

```go
file, err := os.Open("a.txt")
if err != nil {
    return err
}
defer file.Close()
```

执行顺序：

```go
func main() {
    defer fmt.Println("1")
    defer fmt.Println("2")
    defer fmt.Println("3")
}
```

输出：

```text
3
2
1
```

`defer` 是后进先出。

---

## 19. package 包

一个 Go 文件必须属于某个 package。

```go
package main
```

常见项目结构：

```text
demo/
├── go.mod
├── main.go
├── parser/
│   └── csv.go
└── analyzer/
    └── stats.go
```

`parser/csv.go`：

```go
package parser

func Parse() {
}
```

`main.go`：

```go
package main

import "demo/parser"

func main() {
    parser.Parse()
}
```

---

## 20. go mod

Go 使用 `go mod` 管理依赖。

创建项目：

```bash
mkdir go-demo
cd go-demo
go mod init go-demo
```

添加依赖：

```bash
go get github.com/spf13/cobra
```

整理依赖：

```bash
go mod tidy
```

---

## 21. import 导入包

```go
import "fmt"
```

多个包：

```go
import (
    "fmt"
    "os"
    "time"
)
```

如果导入了包但没使用，Go 会编译报错。

---

## 22. goroutine

`goroutine` 是 Go 的轻量级线程。

```go
go func() {
    fmt.Println("running in goroutine")
}()
```

示例：

```go
func main() {
    go fmt.Println("hello")
    time.Sleep(time.Second)
}
```

如果主函数太快结束，goroutine 可能还没执行完程序就退出了。

---

## 23. sync.WaitGroup

用于等待多个 goroutine 执行完成。

```go
var wg sync.WaitGroup

for i := 0; i < 5; i++ {
    wg.Add(1)

    go func(i int) {
        defer wg.Done()
        fmt.Println(i)
    }(i)
}

wg.Wait()
```

说明：

- `wg.Add(1)` 增加一个任务
- `defer wg.Done()` 当前任务完成
- `wg.Wait()` 等待所有任务完成

---

## 24. channel

channel 用于 goroutine 之间通信。

```go
ch := make(chan string)

go func() {
    ch <- "hello"
}()

msg := <-ch
fmt.Println(msg)
```

说明：

- `ch <- "hello"` 发送数据
- `msg := <-ch` 接收数据

带缓冲 channel：

```go
ch := make(chan int, 2)
ch <- 1
ch <- 2
```

关闭 channel：

```go
close(ch)
```

遍历 channel：

```go
for v := range ch {
    fmt.Println(v)
}
```

---

## 25. context

`context` 常用于控制超时、取消任务、传递请求上下文。

```go
ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
defer cancel()
```

示例：

```go
select {
case <-time.After(5 * time.Second):
    fmt.Println("done")
case <-ctx.Done():
    fmt.Println("timeout:", ctx.Err())
}
```

在 Kubernetes、containerd、Kata 源码里，`context.Context` 很常见。

常见函数签名：

```go
func Start(ctx context.Context) error
```

---

## 26. 文件操作

读取文件：

```go
data, err := os.ReadFile("test.txt")
if err != nil {
    fmt.Println(err)
    return
}
fmt.Println(string(data))
```

写文件：

```go
err := os.WriteFile("out.txt", []byte("hello"), 0644)
if err != nil {
    fmt.Println(err)
    return
}
```

打开文件：

```go
file, err := os.Open("test.txt")
if err != nil {
    return
}
defer file.Close()
```

---

## 27. 执行 shell 命令

这对写测试工具很有用。

```go
cmd := exec.Command("ls", "-l")
out, err := cmd.CombinedOutput()
if err != nil {
    fmt.Println("error:", err)
}
fmt.Println(string(out))
```

执行 UnixBench、fio、kubectl、kata-runtime 等命令时会用到。

---

## 28. JSON

结构体转 JSON：

```go
type User struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}

u := User{Name: "Aaron", Age: 25}

data, err := json.Marshal(u)
if err != nil {
    return
}

fmt.Println(string(data))
```

JSON 转结构体：

```go
var u User
err := json.Unmarshal(data, &u)
if err != nil {
    return
}
fmt.Println(u.Name)
```

---

## 29. 单元测试

Go 测试文件以 `_test.go` 结尾。

例如：

```text
add.go
add_test.go
```

`add.go`：

```go
package demo

func Add(a, b int) int {
    return a + b
}
```

`add_test.go`：

```go
package demo

import "testing"

func TestAdd(t *testing.T) {
    got := Add(1, 2)
    if got != 3 {
        t.Fatalf("expected 3, got %d", got)
    }
}
```

运行测试：

```bash
go test ./...
```

---

## 30. 常见命令

```bash
go version        # 查看 Go 版本
go run main.go    # 运行 Go 文件
go build          # 编译项目
go test ./...     # 运行所有测试
go mod init demo  # 初始化 Go 模块
go mod tidy       # 整理依赖
go fmt ./...      # 格式化代码
go vet ./...      # 静态检查
```

---

## 31. Go 和 Java 的简单对照

| Java | Go |
| --- | --- |
| class | struct |
| method | func + receiver |
| implements | 隐式实现 interface |
| exception | error 返回值 |
| try-finally | defer |
| thread | goroutine |
| synchronized | sync.Mutex |
| ArrayList | slice |
| HashMap | map |
| Maven/Gradle | go mod |
| package | package |
| JUnit | testing |

---

## 32. Go 常见代码风格

### 32.1 错误优先返回

```go
result, err := doSomething()
if err != nil {
    return err
}
```

### 32.2 少嵌套

不推荐：

```go
if err == nil {
    if ok {
        doSomething()
    }
}
```

推荐：

```go
if err != nil {
    return err
}

if !ok {
    return nil
}

doSomething()
```

### 32.3 接口小而专

Go 里推荐小接口。

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}
```

不要一上来定义很大的接口。

---

## 33. 你最需要先掌握的部分

如果目标是看 Kata / containerd / Kubernetes 源码，优先学：

1. `struct`
2. `interface`
3. `error`
4. `defer`
5. `context`
6. `goroutine`
7. `channel`
8. `go mod`
9. 文件操作
10. 命令执行
11. JSON / YAML 配置解析
12. 单元测试

---

## 34. 最小练习项目：kata-bench-analyzer

建议写一个小工具：`kata-bench-analyzer`。

目标：

- 读取 CSV
- 按 operation 分组
- 计算平均值
- 输出 Markdown 报告

你会练到：

- 文件读取
- CSV 解析
- struct
- map
- slice
- error
- 函数拆分
- 单元测试

---

## 35. 一个完整小例子

```go
package main

import (
    "encoding/csv"
    "fmt"
    "os"
    "strconv"
)

type Record struct {
    Target    string
    Round     int
    Operation string
    Ops       float64
}

func main() {
    records, err := readCSV("bench.csv")
    if err != nil {
        fmt.Println("read csv failed:", err)
        return
    }

    for _, r := range records {
        fmt.Printf("%s %s round=%d ops=%.2f\n",
            r.Target, r.Operation, r.Round, r.Ops)
    }
}

func readCSV(path string) ([]Record, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    reader := csv.NewReader(file)

    rows, err := reader.ReadAll()
    if err != nil {
        return nil, err
    }

    var records []Record

    for i, row := range rows {
        if i == 0 {
            continue
        }

        round, err := strconv.Atoi(row[1])
        if err != nil {
            return nil, err
        }

        ops, err := strconv.ParseFloat(row[3], 64)
        if err != nil {
            return nil, err
        }

        record := Record{
            Target:    row[0],
            Round:     round,
            Operation: row[2],
            Ops:       ops,
        }

        records = append(records, record)
    }

    return records, nil
}
```

假设 `bench.csv` 内容：

```csv
target,round,operation,ops_per_sec
virtiofs,1,stat,117.39
virtiofs,1,open_close,6933.46
pmem,1,stat,500.21
```

运行：

```bash
go run main.go
```

---

## 36. 学习顺序建议

第一步：

```text
变量、函数、if、for、slice、map
```

第二步：

```text
struct、方法、interface
```

第三步：

```text
error、defer、文件操作、JSON
```

第四步：

```text
goroutine、WaitGroup、channel、context
```

第五步：

```text
写小工具、读源码、补高级语法
```

不要一开始死磕高级语法。先能看懂、能运行、能改代码。
