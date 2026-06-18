# Devmapper、Thin Provisioning、CoW 与 Kata Block Rootfs

> 主题：理解 `thin provisioning`、`CoW`、`block device hotplug/mount`、`thin-pool 管理`，以及它们在 Kata Containers block-based container rootfs 中的关系。

---

## 1. 总体链路

这几个词不是孤立概念，而是在 Kata block-based container rootfs 方案里串成一条链：

```text
container image
  -> containerd devmapper snapshotter
    -> thin-pool
      -> thin provisioning
        -> CoW thin device
          -> Kata block device hotplug
            -> Guest VM /dev/vda
              -> kata-agent mount 成 container /
```

一句话理解：

```text
containerd devmapper 把 container rootfs 做成 thin block device，
Kata 把这个 block device 插进 Guest VM，
kata-agent 把它 mount 成容器的 /。
```

---

## 2. thin-pool 是什么？

`thin-pool` 是 Linux Device Mapper 提供的一种**块设备资源池**。

它不是普通目录，而是一个由块设备组成的池子，通常包含两部分：

```text
thin-pool
├── data device      # 真正存放数据块
└── metadata device  # 记录 thin volume 到真实 block 的映射关系
```

可以把它理解成：

```text
真实磁盘空间：100G
    |
    v
创建一个 thin-pool
    |
    v
从池子里切出很多“看起来很大”的虚拟块设备
```

例如：

```text
thin-pool 实际只有 100G

容器 A 看起来有 10G
容器 B 看起来有 10G
容器 C 看起来有 10G
...
```

这些容器的虚拟块设备不是一创建就立刻占满 10G，而是写入多少，才真正从 pool 里分配多少。

这就是 `thin provisioning`。

---

## 3. thin provisioning 是什么？

`thin provisioning` 可以翻译为**精简配置**或**按需分配空间**。

普通块设备模型：

```text
创建一个 10G 设备
=> 可能直接占用或预留 10G
```

thin provisioning 模型：

```text
创建一个“看起来是 10G”的设备
=> 先不真正占满 10G
=> 写入数据时，才从 thin-pool 里分配真实 block
```

例子：

```text
创建一个逻辑大小 10G 的 container rootfs block device
实际只写了 500M 文件
那么真实可能只占 thin-pool 中约 500M + metadata 开销
```

### 3.1 优点

```text
1. 创建快
2. 空间利用率高
3. 适合大量容器共享一个 pool
4. 不需要每个容器提前分配完整磁盘空间
```

### 3.2 风险

```text
1. pool 看起来还能创建很多设备，但真实空间可能快满了
2. data device 满了，容器写入会失败
3. metadata device 满了，问题更严重，可能影响整个 pool
4. 需要持续监控和扩容
```

thin provisioning 最大的坑是：

```text
逻辑上分出去的空间 > 实际物理空间
```

这叫 `over-provisioning`。

---

## 4. CoW 是什么？

`CoW` 是 `Copy-on-Write`，即**写时复制**。

核心思想：

```text
能共享就共享；
只有发生写入时，才复制一份出来修改。
```

容器镜像非常适合这个模型。

比如 `busybox` 镜像本身是只读层：

```text
busybox image layer  # 只读
```

启动多个容器：

```text
container A
container B
container C
```

一开始它们可以共享同一份只读镜像块：

```text
container A ─┐
container B ─┼──> busybox base blocks
container C ─┘
```

如果 container A 修改某个文件，例如 `/etc/hosts`：

```text
container A 写 block X
```

不会直接修改共享的 base block，而是：

```text
1. 先把原 block X 复制到 container A 自己的 writable layer
2. 再在 container A 自己那份 block 上修改
3. container B / C 仍然看到原来的 base block
```

图示：

```text
写之前：

container A ─┐
container B ─┼──> base block X
container C ─┘


container A 写入后：

container A ───> A 自己的新 block X'
container B ─┐
container C ─┴──> base block X
```

---

## 5. block-level CoW 和 overlayfs CoW 的区别

普通 `overlayfs` 更偏文件级：

```text
文件 A 被改
=> copy-up 文件 A 到 upperdir
=> 修改 upperdir 里的文件
```

`devmapper thin device` 是块级：

```text
某个 block 被改
=> copy-on-write 这个 block
=> 修改这个 block
```

这对 Kata 很关键。

```text
overlayfs snapshotter:
  Host 上准备出一个目录
  Kata 需要用 virtio-fs 把目录共享进 VM

devmapper snapshotter:
  Host 上准备出一个块设备
  Kata 可以把块设备直接 hotplug 到 VM
```

对比：

```text
默认 overlayfs snapshotter：

container image
   |
   v
containerd overlayfs
   |
   v
Host directory rootfs
   |
   v
virtio-fs
   |
   v
Guest VM
   |
   v
container /


devmapper snapshotter：

container image
   |
   v
containerd devmapper
   |
   v
Host block device
   |
   v
virtio-scsi / virtio-blk
   |
   v
Guest VM /dev/vda
   |
   v
container /
```

---

## 6. block device hotplug 是什么？

`hotplug` 是指：**VM 已经启动后，再动态插入一个虚拟设备**。

类比物理机：

```text
电脑正在运行
插入一个 U 盘
系统发现 /dev/sdb
```

Kata 中类似：

```text
Guest VM 已经启动
Kata runtime 创建容器
发现这个容器 rootfs 是 block device
于是通过 QEMU / Cloud Hypervisor 把这个 block device 热插进 VM
Guest VM 里出现 /dev/vda 或 /dev/vdb
kata-agent 再把它 mount 成容器 /
```

流程：

```text
Host devmapper thin device
        |
        | hotplug
        v
QEMU / hypervisor
        |
        v
Guest VM sees /dev/vda
        |
        v
kata-agent mount /dev/vda
        |
        v
container /
```

---

## 7. mount 是什么？

`mount` 是指：**把一个设备里的文件系统挂到某个目录上**。

例如 Guest VM 里出现：

```text
/dev/vda
```

这个设备中有 ext4/xfs 文件系统，`kata-agent` 会把它挂载成容器 rootfs：

```bash
mount /dev/vda /run/kata-containers/shared/containers/<id>/rootfs
```

容器进程启动时，把这个目录作为 `/`：

```text
container /
  -> 实际来自 /dev/vda
```

验证命令：

```bash
findmnt -T / -o TARGET,SOURCE,FSTYPE,OPTIONS
```

如果看到：

```text
/  /dev/vda  ext4  rw,...
```

说明：

```text
容器 rootfs 是 block-based
```

如果看到：

```text
/  none  virtiofs  rw,...
```

说明：

```text
容器 rootfs 还是 virtio-fs 共享目录
```

---

## 8. thin-pool 管理是什么？

因为 devmapper 使用共享块设备池，所以需要管理这个池子的健康状态。

核心管理内容包括：

```text
1. data 空间
2. metadata 空间
3. thin device 生命周期
4. pool 扩容
5. 残留设备清理
6. pool 状态监控
```

---

## 9. 管 data 空间

`data device` 存真实数据块。

如果 data 满了：

```text
容器写入失败
文件系统可能报 I/O error
业务可能异常
```

常用查看命令：

```bash
sudo dmsetup status
sudo lvs -a -o+seg_monitor,data_percent,metadata_percent
```

重点看：

```text
Data%
```

---

## 10. 管 metadata 空间

`metadata device` 记录 block 映射关系，例如：

```text
thin volume A 的逻辑 block 100
实际对应 pool 里的物理 block 5678
```

如果 metadata 满了，比 data 满更麻烦，因为整个 pool 的映射关系都依赖它。

重点看：

```text
Meta%
```

---

## 11. 管 thin device 生命周期

每个容器、每个 snapshot 都可能对应一个 thin device。

容器删除后，要确保：

```text
snapshot 被删除
thin device 被释放
dm 设备被清理
```

否则可能出现：

```text
1. pool 空间泄漏
2. dmsetup 里残留很多设备
3. containerd 再创建 snapshot 失败
```

常用查看命令：

```bash
sudo dmsetup ls
sudo dmsetup info
sudo ctr -n k8s.io snapshots --snapshotter devmapper ls
```

---

## 12. 管扩容

如果 pool 快满，要扩容：

```text
1. 扩 data device
2. 扩 metadata device
3. resize thin-pool
```

生产环境一般不会使用 loopback 文件，而是使用：

```text
真实磁盘 / 裸块设备
LVM thin-pool
```

测试可以用 loopback，但性能和稳定性不能代表生产。

---

## 13. 它们在 Kata block rootfs 里的关系

完整关系：

```text
1. containerd devmapper snapshotter 创建 thin-pool

2. 拉取镜像时：
   image layer 被 unpack 到 devmapper thin snapshot

3. 创建容器时：
   为容器生成一个 writable thin device

4. 这个 thin device 背后是：
   base image blocks + container writable CoW blocks

5. Kata runtime 发现 rootfs 是 block device

6. Kata 通过 virtio-scsi / virtio-blk hotplug 到 Guest VM

7. Guest VM 里出现 /dev/vda

8. kata-agent mount /dev/vda 作为 container /

9. 容器进程启动
```

架构图：

```text
Host
┌────────────────────────────────────────────┐
│ containerd devmapper snapshotter            │
│                                            │
│ thin-pool                                  │
│ ├── base image thin devices                │
│ ├── snapshot thin devices                  │
│ └── container writable thin device         │
│                    │                       │
└────────────────────┼───────────────────────┘
                     │ hotplug
                     v
Guest VM
┌────────────────────────────────────────────┐
│ /dev/vda                                   │
│   │                                        │
│   v                                        │
│ mounted as container rootfs                │
└────────────────────────────────────────────┘
                     │
                     v
Container
┌────────────────────────────────────────────┐
│ /                                          │
│ app / bash / nginx                         │
└────────────────────────────────────────────┘
```

---

## 14. 性能和开销怎么理解？

### 14.1 可能变快的地方

相比 virtio-fs：

```text
virtio-fs:
container open/stat/read
  -> guest virtio-fs
  -> virtiofsd
  -> host filesystem
  -> overlayfs directory
```

block rootfs：

```text
block rootfs:
container open/stat/read
  -> guest VFS
  -> guest ext4/xfs
  -> virtio-blk/scsi
  -> host block backend
```

因此 block rootfs 可能减少：

```text
1. virtiofsd 用户态转发
2. Host/Guest 文件级元数据交互
3. overlayfs 文件级 copy-up 路径
```

对这些场景可能有利：

```text
1. 大量小文件
2. 大量 stat/open/readdir
3. 代码仓库扫描
4. 依赖目录遍历
5. 容器 rootfs 读写频繁
```

---

### 14.2 可能变慢或增加开销的地方

`devmapper` / `thin-pool` 不是零成本：

```text
1. 第一次写某个 block 时有 CoW 开销
2. thin provisioning 需要维护 block 映射 metadata
3. 随机写多了可能碎片化
4. 创建容器时可能有 block device hotplug 开销
5. agent 还要等待设备出现并 mount
6. thin-pool 需要监控、清理、扩容
```

所以它不是绝对优于 virtio-fs：

```text
元数据/小文件密集场景：
    block rootfs 可能更好

启动极致优化场景：
    hotplug/mount 可能增加启动耗时

写入非常重的场景：
    thin provisioning + CoW 可能带来额外开销

运维简单性：
    virtio-fs 明显更简单
```

---

## 15. 和 Kata virtio-fs 路径的核心区别

```text
virtio-fs rootfs：
  Host directory
    -> virtiofsd
      -> Guest virtio-fs mount
        -> container /

block-based rootfs：
  Host devmapper thin device
    -> virtio-blk / virtio-scsi hotplug
      -> Guest /dev/vda
        -> kata-agent mount
          -> container /
```

关键区别：

```text
virtio-fs 是文件系统目录共享；
block-based rootfs 是块设备映射。
```

---

## 16. 一句话总结

```text
thin-pool 是块设备资源池；
thin provisioning 是从池子里按需分配真实 block；
CoW 是镜像层和容器层共享，写入时才复制 block；
block device hotplug 是 Kata 在 VM 运行后把容器 rootfs 块设备插进 Guest；
mount 是 kata-agent 在 Guest 内把 /dev/vda 挂成容器 /；
thin-pool 管理就是监控和维护这个块设备池的 data、metadata、生命周期和扩容。
```

放到 Kata 里：

```text
containerd devmapper 把 container rootfs 做成 thin block device，
Kata 把这个 block device 插进 Guest VM，
kata-agent 把它 mount 成容器的 /。
```
