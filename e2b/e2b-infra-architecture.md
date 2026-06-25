# E2B Infra 源码架构总结

> 源码仓库：`YIYUANYUAN1116/infra`  
> 主题：E2B Cloud 基础设施架构、核心服务、源码阅读入口

## 1. 一句话理解

`infra` 仓库不是普通后端项目，而是 E2B Cloud 的基础设施仓库。

它负责把用户通过 SDK / CLI 发起的 sandbox 请求，经过 API 控制面、Nomad/Consul 调度、Orchestrator 节点、Firecracker MicroVM，最终落到 sandbox 内的 `envd` daemon 上执行。

可以简化成：

```text
Terraform/Packer 负责造集群和机器镜像
Nomad/Consul 负责跑服务和服务发现
API 负责控制面
Orchestrator 负责节点侧创建 Firecracker VM
Envd 负责 VM 内执行命令、文件、端口、cgroup
Client Proxy 负责把用户流量路由到具体 sandbox
Postgres/Redis/GCS/S3/ClickHouse/Loki/OTEL 负责状态、缓存、对象、事件和可观测性
```

## 2. 总体架构

```text
用户 / SDK / CLI
    |
    v
Cloudflare / Ingress / Traefik
    |
    +---------------------------+
    | Control Plane             |
    |                           |
    |  API / Dashboard API      |
    |  - Auth / Team / APIKey   |
    |  - Sandbox 生命周期管理    |
    |  - Template Build 管理     |
    |  - OpenAPI + Gin + gRPC   |
    +---------------------------+
        |       |        |
        |       |        +--> Postgres：业务数据 / 用户 / team / template
        |       +----------> Redis：catalog / rate limit / events
        +------------------> Nomad / Consul：调度与服务发现
                         |
                         v
              +------------------------+
              | Orchestrator Nodes     |
              |                        |
              | - Firecracker VM 创建  |
              | - 网络池 / NBD 设备池  |
              | - sandbox proxy        |
              | - egress firewall      |
              | - template cache       |
              | - volume / chunk svc   |
              +------------------------+
                         |
                         v
              +------------------------+
              | Firecracker MicroVM    |
              |                        |
              | envd daemon            |
              | - process exec         |
              | - filesystem API       |
              | - cgroup 控制          |
              | - port forward         |
              +------------------------+
```

## 3. 仓库目录分层

`go.work` 表明这是一个 Go workspace，核心模块包括：

```text
iac/
  provider-gcp/
  provider-aws/
  nomad / k8s / cluster / redis / storage ...

packages/
  api/                 控制面 API
  dashboard-api/       控制台 API
  client-proxy/        用户访问 sandbox 的流量入口
  orchestrator/        节点侧 sandbox 管理器，核心
  envd/                sandbox 内部 daemon
  db/                  Postgres 迁移 + sqlc
  clickhouse/          事件 / 统计数据
  shared/              公共 grpc、storage、logger、telemetry、feature flag
  auth/                认证鉴权
```

外层 `Makefile` 是总控入口，会根据 `PROVIDER` 选择 `iac/provider-gcp` 或 `iac/provider-aws`，并提供：

```text
make init
make plan
make apply
make plan-without-jobs
make build-and-upload
make copy-public-builds
make migrate
make prep-cluster
```

其中 `build-and-upload` 会构建并上传：

```text
api
client-proxy
dashboard-api
docker-reverse-proxy
clean-nfs-cache
orchestrator
template-manager
envd
clickhouse-migrator
nomad-nodepool-apm
```

## 4. 控制面：API 服务

源码入口：

```text
packages/api/main.go
```

API 服务主要负责：

1. 提供 OpenAPI HTTP 接口。
2. 管理 sandbox 生命周期：创建、连接、暂停、恢复、快照、删除等。
3. 管理 template build。
4. 处理 Auth / Team / API Key / Admin Token。
5. 访问 Postgres、Redis、Nomad、ClickHouse。
6. 对外暴露 HTTP，对内暴露 gRPC。

API 入口里会启动 Gin Server，并挂载：

```text
Tracing middleware
Metrics middleware
Logging middleware
Request timeout
CORS
OpenAPI request validator
AuthenticationFunc
Rate limit
Blocked team middleware
Generated OpenAPI handlers
```

API 同时启动两类 gRPC：

```text
API internal gRPC  -> 给集群内部 client-proxy / orchestrator 调用
API edge gRPC      -> 给边缘 client-proxy 通过 OIDC 调用
```

可以理解为：

```text
用户请求进来
  -> API 做认证、校验、限流
  -> 查 DB / Redis
  -> 通过 Nomad / Orchestrator 创建或操作 sandbox
  -> 返回 sandbox 连接信息
```

## 5. 数据面核心：Orchestrator

源码入口：

```text
packages/orchestrator/main.go
packages/orchestrator/pkg/factories/run.go
```

`orchestrator/main.go` 很薄，核心只是调用：

```go
factories.Run(factories.Options{
    Version:       version.Version,
    CommitSHA:     commitSHA,
    EgressFactory: defaultEgressFactory,
})
```

真正复杂逻辑在 `pkg/factories/run.go`。

Orchestrator 启动时会初始化：

```text
telemetry / logger
feature flags
storage provider
Redis
template cache
ClickHouse event delivery
cgroup manager
sandbox observer
host metrics poller
sandbox proxy
egress proxy
NBD device pool
network pool
sandbox factory
volume service
orchestrator gRPC service
template manager
NFS proxy
hyperloop server
HTTP health server
pprof server
```

核心职责：

```text
API 发请求：创建 / 暂停 / 恢复 / 删除 sandbox
    |
    v
Nomad / service discovery 找到合适 orchestrator
    |
    v
orchestrator:
  - 分配网络 slot
  - 分配 NBD / block device
  - 拉取 template / snapshot
  - 启动 Firecracker microVM
  - 维护 sandbox 路由表
  - 上报事件 / 指标
```

这个模块是整个仓库最值得深入看的地方，尤其适合研究 Firecracker、网络、块设备、sandbox 生命周期。

## 6. Sandbox 内部：envd

源码入口：

```text
packages/envd/main.go
```

`envd` 是跑在 Firecracker VM 内部的 daemon，默认端口是 `49983`。

它主要提供：

```text
process exec      进程执行
filesystem API    文件系统访问
cgroup 控制       资源隔离 / 限制
port scanner      端口扫描
port forwarder    端口转发
MMDS poll         从 Firecracker MMDS 获取配置
```

可以理解为：

```text
Orchestrator 管 VM 生命周期
Envd 管 VM 里面的用户进程、文件、端口、资源限制
```

Envd 里还有一个比较重要的点：它会按进程类型设置 cgroup 权重，例如：

```text
PTY process
socat process
user process
```

这样可以避免用户进程把 sandbox 内资源打满。

## 7. Client Proxy：用户流量入口

源码入口：

```text
packages/client-proxy/main.go
```

Client Proxy 负责把用户访问 sandbox 的 HTTP 流量转发到对应 Orchestrator 节点。

它依赖 Redis catalog：

```text
sandboxID -> orchestrator/node
```

流量链路：

```text
用户浏览器 / SDK
    |
    v
client-proxy
    |
    |  查 Redis catalog：sandboxID -> orchestrator/node
    |
    v
orchestrator sandbox proxy
    |
    v
Firecracker VM / envd / 用户进程端口
```

它还有 `PausedSandboxResumer`，当用户访问一个暂停的 sandbox 时，可以通过 API gRPC 触发恢复流程。

## 8. IaC / 部署架构

部署主要靠 Terraform + Packer。

自托管需要：

```text
Packer      构建 orchestrator client/server 磁盘镜像
Terraform   创建云资源
Golang      构建 Go 服务
Docker      构建镜像
Docker Buildx
NPM
Cloudflare account/domain
PostgreSQL database
```

### GCP

GCP 部署会用到：

```text
Secret Manager
Certificate Manager
Compute Engine
Artifact Registry
OS Config
Stackdriver Monitoring
Stackdriver Logging
Filestore
GCS Bucket
Nomad / Consul
```

主要 Terraform 模块：

```text
module.init       基础资源：bucket、secret、registry 等
module.cluster    Nomad/Consul 节点集群
module.k8s_apps   K8s app，namespace e2b
module.nomad      Nomad jobs：api、proxy、orchestrator、template-manager 等
module.redis      可选托管 Redis
module.remote_repository 可选远程镜像仓库
```

### AWS

AWS 部署会创建：

```text
S3 bucket for Terraform state
VPC / subnets / networking
ECR repositories
S3 buckets for templates / kernels / builds / backups
AWS Secrets Manager
Cloudflare DNS / TLS
Nomad cluster node AMI
```

AWS 架构里有几类 Node Pool：

```text
Control Server   Nomad / Consul servers
API              API server、ingress、client proxy、otel、loki、logs collector
Client           Firecracker orchestrator nodes
Build            Template manager / sandbox template build
ClickHouse       Analytics database
```

## 9. 数据与依赖组件

| 组件 | 作用 |
|---|---|
| Postgres | 用户、团队、template、sandbox 元数据等业务数据 |
| Redis | sandbox catalog、rate limit、events、cache/registry |
| Nomad | 服务部署与调度 |
| Consul | 服务发现 / ACL |
| Firecracker | microVM sandbox 运行时 |
| GCS / S3 | template、kernel、firecracker binary、build cache、snapshot 等对象存储 |
| ClickHouse | sandbox event、host stats、分析类数据 |
| Loki / OTEL | 日志、指标、trace |
| Cloudflare | DNS / TLS / 入口域名 |

## 10. 源码阅读建议

如果目标是理解整体平台：

```text
README.md
self-host.md
go.work
Makefile
iac/provider-gcp/main.tf
iac/provider-aws/main.tf
```

如果目标是理解控制面：

```text
packages/api/main.go
packages/api/internal/handlers/
packages/auth/
packages/db/
packages/shared/pkg/grpc/
```

如果目标是理解 Firecracker sandbox 生命周期：

```text
packages/orchestrator/main.go
packages/orchestrator/pkg/factories/run.go
packages/orchestrator/pkg/sandbox/
packages/orchestrator/pkg/sandbox/network/
packages/orchestrator/pkg/sandbox/nbd/
packages/orchestrator/pkg/sandbox/block/
packages/orchestrator/pkg/proxy/
packages/orchestrator/pkg/tcpfirewall/
packages/orchestrator/pkg/nfsproxy/
```

如果目标是理解 VM 内部执行：

```text
packages/envd/main.go
packages/envd/internal/services/process/
packages/envd/internal/services/filesystem/
packages/envd/internal/services/cgroups/
packages/envd/internal/port/
```

## 11. 和 Kata / 容器虚拟化学习的对应关系

这个项目和 Kata 的关注点很像，但运行时不同：

| 方向 | E2B infra | Kata Containers |
|---|---|---|
| 虚拟化运行时 | Firecracker | QEMU / Cloud Hypervisor / Firecracker 等 |
| 控制面 | API + Nomad + Orchestrator | Kubernetes + containerd + kata-runtime |
| VM 内 agent | envd | kata-agent |
| rootfs/template | template cache + object storage | OCI image / rootfs / snapshotter |
| 网络 | orchestrator network pool + proxy | CNI + tap/veth/macvtap 等 |
| 存储 | NBD / template / volume / NFS proxy | virtio-fs / virtio-blk / virtio-scsi / pmem |
| 用户访问 | client-proxy | 一般由 Pod Service / ingress 暴露 |

对你现在研究 Kata + QEMU 性能来说，E2B 里最有价值的参考是：

```text
1. Orchestrator 如何抽象 sandbox 生命周期
2. 如何做网络池 / 设备池预分配
3. 如何处理 sandbox proxy 和端口转发
4. 如何把 VM 内 agent/envd 和控制面连接起来
5. 如何做 template cache / snapshot / object storage
6. 如何做事件、日志、metrics 可观测性
```

## 12. 总结

E2B infra 可以按三层看：

```text
控制面：API / Dashboard API / Auth / DB / Redis
节点面：Orchestrator / Template Manager / Client Proxy / Nomad jobs
执行面：Firecracker VM / envd / 用户进程 / 文件系统 / cgroup / 端口转发
```

最核心的调用链：

```text
SDK/CLI
  -> API
  -> Nomad/Consul 找节点
  -> Orchestrator 创建 Firecracker VM
  -> VM 内启动 envd
  -> client-proxy 路由用户流量
  -> envd 执行命令 / 文件操作 / 端口转发
```

这套架构本质上是一个面向 AI Agent 的 sandbox 云平台。它把 Firecracker MicroVM 包装成可通过 SDK 创建、连接、暂停、恢复、快照、销毁的云服务。
