# 真实块设备、LVM thin-pool 与 loop device 对比

## 1. 背景

在测试 Kata Containers 的 block-based container rootfs 时，containerd devmapper snapshotter 需要一个 Device-mapper thin-pool 作为后端。

这个 thin-pool 可以基于不同类型的底层存储创建，常见有三种：

1. 真实块设备
2. LVM thin-pool
3. loop device

它们都能为 devmapper snapshotter 提供块存储能力，但适用场景、性能真实性、配置复杂度和生产可用性不同。

---

## 2. 一句话理解

```text
真实块设备：
  真磁盘、真分区、云盘、NVMe 盘，性能最真实。

LVM：
  基于真实块设备的一层逻辑卷管理，方便切分、扩容和创建 thin-pool，适合正式环境。

loop device：
  用普通文件伪装成块设备，配置最简单，适合开发测试，但不适合生产性能结论。
```

从性能真实性看：

```text
真实块设备 > LVM thin-pool > loop device
```

从测试方便程度看：

```text
loop device > LVM thin-pool > 真实块设备
```

---

## 3. 真实块设备

真实块设备指 Linux 系统中真实存在的磁盘、分区、云盘或虚拟磁盘。

常见示例：

```text
/dev/sdb
/dev/vdb
/dev/nvme1n1
/dev/nvme0n1p3
```

查看命令：

```bash
lsblk
lsblk -f
```

示例输出：

```text
NAME        SIZE TYPE MOUNTPOINT
sda         100G disk
├─sda1        1G part /boot
└─sda2       99G part /
sdb         200G disk
nvme1n1     500G disk
```

其中 `/dev/sdb`、`/dev/nvme1n1` 就是可用于测试的真实块设备。

### 3.1 在 Kata devmapper 测试中的路径

```text
真实磁盘 /dev/sdb
  -> dmsetup thin-pool
    -> containerd devmapper snapshotter
      -> Kata runtime
        -> Guest VM 内虚拟块设备 /dev/sdX
          -> container rootfs /
```

### 3.2 优点

```text
性能最真实
路径最短
适合严肃性能测试或生产环境
没有 loop 文件和宿主机文件系统额外开销
```

### 3.3 缺点

```text
需要空闲磁盘或空闲分区
操作风险高，容易清空已有数据
扩容和管理不如 LVM 灵活
```

### 3.4 适用场景

```text
生产环境
准生产性能测试
对 I/O 性能结论要求较高的测试
```

---

## 4. LVM thin-pool

LVM 全称是 Logical Volume Manager，即逻辑卷管理器。

它不是一种新硬盘，而是建立在真实块设备上的一层磁盘管理抽象。

可以理解为：

```text
把一块或多块真实磁盘加入一个磁盘池，
再从这个磁盘池中切出逻辑卷使用。
```

LVM 中有三个核心概念：

| 名称 | 全称 | 含义 | 类比 |
|---|---|---|---|
| PV | Physical Volume | 物理卷，一般来自真实磁盘或分区 | 原材料 |
| VG | Volume Group | 卷组，多个 PV 组成的存储池 | 总水池 |
| LV | Logical Volume | 逻辑卷，从 VG 中切出的卷 | 小水箱 |

关系：

```text
真实块设备 /dev/sdb
        |
        v
PV: physical volume
        |
        v
VG: kata_vg
        |
        v
LV: kata_thinpool
```

实际设备名通常类似：

```text
/dev/mapper/kata_vg-kata_thinpool
```

### 4.1 在 Kata devmapper 测试中的路径

```text
真实磁盘 /dev/sdb
  -> LVM PV
    -> LVM VG
      -> LVM thin-pool LV
        -> containerd devmapper snapshotter
          -> Kata runtime
            -> Guest VM 内虚拟块设备 /dev/sdX
              -> container rootfs /
```

### 4.2 优点

```text
基于真实块设备，性能更接近正式环境
比直接裸盘管理更灵活
支持扩容
适合创建 thin-pool
适合长期测试和正式环境
```

### 4.3 缺点

```text
仍然需要真实空闲磁盘或分区
配置比 loop device 稍复杂
需要理解 PV、VG、LV、thin-pool 等概念
```

### 4.4 适用场景

```text
正式环境推荐方式
准生产性能测试
需要长期维护 devmapper pool 的场景
```

---

## 5. loop device

loop device 是一种把普通文件映射成块设备的机制。

例如先创建两个普通文件：

```text
/var/lib/containerd/devmapper/data
/var/lib/containerd/devmapper/meta
```

再通过 `losetup` 映射成：

```text
/dev/loop0
/dev/loop1
```

系统就会把 `/dev/loop0`、`/dev/loop1` 当成块设备使用。

### 5.1 示例

```bash
truncate -s 50G /var/lib/containerd/devmapper/data
truncate -s 5G  /var/lib/containerd/devmapper/meta

DATA_DEV=$(losetup --find --show /var/lib/containerd/devmapper/data)
META_DEV=$(losetup --find --show /var/lib/containerd/devmapper/meta)
```

然后可以基于这两个 loop 设备创建 devmapper thin-pool。

### 5.2 在 Kata devmapper 测试中的路径

```text
普通文件 /var/lib/containerd/devmapper/data
  -> 宿主机文件系统
    -> loop device /dev/loop0
      -> dmsetup thin-pool
        -> containerd devmapper snapshotter
          -> Kata runtime
            -> Guest VM 内虚拟块设备 /dev/sdX
              -> container rootfs /
```

### 5.3 优点

```text
最方便
不需要空闲磁盘
适合快速验证功能
适合开发测试
容易清理和重建
```

### 5.4 缺点

```text
性能不真实
I/O 多经过宿主机文件系统和 loop 映射
延迟和抖动可能更大
重启后 loop 映射和 dmsetup 映射可能丢失
不推荐生产
不适合严肃性能结论
```

### 5.5 适用场景

```text
功能验证
快速实验
验证 Kata 是否可以跑 block-based container rootfs
验证 containerd devmapper snapshotter 配置是否正确
```

---

## 6. 三者对比表

| 对比项 | 真实块设备 | LVM thin-pool | loop device |
|---|---|---|---|
| 底层来源 | 真实磁盘、分区、云盘 | 真实磁盘上的 LVM 逻辑卷 | 普通文件 |
| 设备示例 | `/dev/sdb`、`/dev/nvme1n1` | `/dev/mapper/kata_vg-kata_thinpool` | `/dev/loop0` |
| 是否需要空闲磁盘 | 需要 | 需要 | 不需要 |
| 配置复杂度 | 中等 | 中等偏高 | 最简单 |
| 管理灵活性 | 一般 | 高 | 低 |
| 性能真实性 | 最高 | 高 | 较低 |
| 是否适合功能验证 | 可以 | 可以 | 最适合 |
| 是否适合性能测试 | 适合 | 适合 | 仅适合初步观察 |
| 是否适合生产 | 可以 | 推荐 | 不推荐 |
| 重启后稳定性 | 稳定 | 稳定 | 需要重新 losetup/dmsetup |
| 扩容能力 | 一般 | 好 | 不推荐扩容 |

---

## 7. 在当前 Kata 测试中的含义

### 7.1 当前 loopback devmapper 方案

当前测试中使用的 loopback 方式大致为：

```text
/var/lib/containerd/devmapper/data 普通文件
/var/lib/containerd/devmapper/meta 普通文件
        |
        v
/dev/loop0、/dev/loop1
        |
        v
kata-devpool
        |
        v
containerd devmapper snapshotter
        |
        v
Kata container rootfs
```

该方案已经可以验证：

```text
containerd devmapper snapshotter 可用
Kata runtime 可以使用 devmapper snapshotter
容器 / 可以从 virtiofs 切换为 /dev/sdX ext4
block-based container rootfs 功能链路可行
```

但是它不能代表生产性能，因为 I/O 路径中混入了：

```text
loop device
宿主机文件系统
普通文件 backing store
```

### 7.2 LVM thin-pool 正式测试方案

更正式的测试应改为：

```text
/dev/sdb 真实磁盘
        |
        v
LVM PV / VG
        |
        v
/dev/mapper/kata_vg-kata_thinpool
        |
        v
containerd devmapper snapshotter
        |
        v
Kata container rootfs
```

这种方式更接近正式环境，性能结论更可信。

### 7.3 真实块设备裸 dmsetup 方案

也可以不用 LVM，直接用两个真实分区：

```text
/dev/sdb1  metadata device
/dev/sdb2  data device
        |
        v
dmsetup thin-pool
        |
        v
containerd devmapper snapshotter
        |
        v
Kata container rootfs
```

这种方式路径更直接，但管理不如 LVM 方便，重启后需要确保 dmsetup 映射恢复。

---

## 8. 为什么 loop device 不适合生产性能结论

loop device 的核心问题是：它用普通文件伪装块设备。

因此 I/O 路径更长：

```text
container /dev/sdX
  -> guest ext4
  -> virtio-scsi / virtio-blk
  -> host devmapper thin-pool
  -> loop device
  -> host filesystem
  -> real disk
```

相比真实块设备路径：

```text
container /dev/sdX
  -> guest ext4
  -> virtio-scsi / virtio-blk
  -> host devmapper thin-pool
  -> real block device
```

loopback 多了：

```text
loop device
host filesystem
普通文件映射
```

这会带来：

```text
额外路径开销
更高延迟
更大抖动
flush/fsync 路径更复杂
discard/TRIM 结果不稳定
```

所以 loopback 适合验证功能，不适合做生产性能结论。

---

## 9. 选型建议

### 9.1 只想验证 block-based rootfs 能否跑通

选择 loop device。

目标：

```text
验证 containerd devmapper snapshotter 是否能启动
验证 Kata 容器 / 是否变成 /dev/sdX ext4
验证 block-based container rootfs 链路是否可用
```

结论可以写：

```text
loopback devmapper 方案验证了功能可行，但不代表生产性能。
```

### 9.2 想做较可信的性能测试

选择 LVM thin-pool。

目标：

```text
减少 loop 文件和宿主机文件系统干扰
获得更接近正式环境的 block rootfs 性能数据
```

结论可以写：

```text
LVM thin-pool 基于真实块设备，更适合作为 devmapper snapshotter 的正式测试后端。
```

### 9.3 真正生产环境

选择真实块设备或 LVM thin-pool。

更推荐 LVM thin-pool，因为：

```text
管理方便
可扩容
可维护性更好
适合长期使用
```

---

## 10. 实验记录建议

进行 Kata block-based rootfs 测试时，应记录以下信息：

```bash
findmnt -T / -o TARGET,SOURCE,FSTYPE,OPTIONS
lsblk
ctr plugins ls | grep devmapper
containerd config dump | grep -n "devmapper\|snapshotter\|kata" -C 5
dmsetup ls
lvs -a
```

核心判断标准：

```text
容器内 / 挂载为 /dev/sdX ext4：
  block-based container rootfs 生效。

容器内 / 挂载为 none virtiofs：
  仍然是 virtio-fs rootfs，block-based rootfs 未生效。
```

---

## 11. 总结

```text
真实块设备：
  真磁盘或真分区，路径短，性能最真实，适合生产和严肃性能测试。

LVM thin-pool：
  基于真实块设备的逻辑卷管理方式，兼顾性能真实性和管理灵活性，推荐用于正式环境。

loop device：
  用普通文件伪装块设备，最适合快速开发测试和功能验证，但不适合生产性能结论。
```

对当前 Kata Containers devmapper 测试来说：

```text
loop device：
  用于验证 block-based rootfs 链路能否跑通。

LVM thin-pool：
  用于更可信的性能测试。

真实块设备：
  用于生产或最接近真实硬件路径的测试。
```
