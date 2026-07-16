# Kata Containers：Cloud Hypervisor 从 virtio-pmem 切换到 virtio-blk 的源码调查

## 1. 调查目标

本调查回答以下问题：

1. Cloud Hypervisor 在 Kata Containers 中何时停止默认使用 virtio-pmem 承载 Guest image？
2. 何时改为默认使用 virtio-blk？
3. 该变化是否由 CVE-2026-24834 引起？
4. 为什么 QEMU 路径仍可继续使用 NVDIMM？
5. `disable_image_nvdimm`、`block_device_driver` 和 Guest rootfs 之间是什么关系？

默认调查仓库：

- Repository: `kata-containers/kata-containers`
- 对比版本：`3.26.0`、`3.27.0`
- 当前代码参考：`main`

---

## 2. 结论

### 2.1 版本结论

从 `3.26.0` 与 `3.27.0` 的源码差异可以确认：

- Kata 3.26.0 的 Cloud Hypervisor 配置仍默认允许使用 PMEM/virtio-pmem 承载 Guest image。
- Kata 3.27.0 为 Cloud Hypervisor 增加了专用默认值：

```makefile
DEFDISABLEIMAGENVDIMM_CLH ?= true
```

- Kata 3.27.0 的 Go runtime 还在 Cloud Hypervisor backend 的 `setConfig()` 中强制执行：

```go
clh.config.DisableImageNvdimm = true
```

因此，Kata 3.27.0 中 Cloud Hypervisor 的 Guest image 不再进入 `PmemConfig` 分支，而是进入只读 `DiskConfig` 分支，通过 virtio-blk 提供给 Guest。

这不是单纯修改配置文件默认值，而是 runtime 层的强制关闭。

### 2.2 CVE 关系结论

从修复行为、版本边界和漏洞攻击链看，该变化与 CVE-2026-24834 的缓解目标一致：

```text
旧路径：
Host kata-containers.img
→ Cloud Hypervisor virtio-pmem
→ Guest /dev/pmem0
→ 原始 PMEM 设备可能被访问
→ 共享 Guest image 存在被修改风险

新路径：
Host kata-containers.img
→ Cloud Hypervisor readonly DiskConfig
→ virtio-blk
→ Guest 只读块设备
```

但是，截至本次调查，公开 GitHub commit message 和 PR 描述中没有检索到一条明确写着：

```text
Fix CVE-2026-24834
```

因此文档中的准确表述应为：

> 已从源码确认 3.27.0 切断了 Cloud Hypervisor 的 Guest image virtio-pmem 路径；该变化与 CVE-2026-24834 的修复目标高度吻合。但尚未从公开 commit message 中确认官方将该具体提交直接标记为 CVE 修复提交。

### 2.3 QEMU 结论

QEMU 仍保留 NVDIMM，核心原因不是“NVDIMM 天然安全”，而是 QEMU 与 Kata 已实现真正的只读 NVDIMM 后端：

```text
readonly=on
+ unarmed=on
```

这能够在 Hypervisor memory backend 层阻止持久修改，而不是仅依赖 Guest 文件系统的只读 mount。

---

## 3. 概念边界

### 3.1 Cloud Hypervisor 使用的是 virtio-pmem

历史上的 Cloud Hypervisor Guest image PMEM 路径为：

```text
Host image 文件
→ Cloud Hypervisor PmemConfig
→ virtio-pmem
→ Guest virtio_pmem 驱动
→ Linux libnvdimm
→ /dev/pmem0
→ Guest rootfs
```

### 3.2 QEMU 使用的是 NVDIMM

QEMU 路径为：

```text
Host image 文件
→ QEMU memory-backend-file
→ QEMU -device nvdimm
→ Guest ACPI/NFIT、libnvdimm
→ /dev/pmem0
→ Guest rootfs
```

两者最终都可能在 Guest 中表现为 `/dev/pmem0`，但 Hypervisor 设备模型、Guest 枚举方式和只读保护机制不同。

---

## 4. 历史时间线

## 4.1 早期：Cloud Hypervisor 支持 virtio-pmem 启动

2020 年，Kata 为 Guest kernel 启用 `CONFIG_VIRTIO_PMEM`，提交说明明确表示该配置用于 Cloud Hypervisor 从 PMEM 启动：

- Commit: `fbad186abec20279af94c5d85aab0ef20d6cf997`
- Title: `kernel: Enable CONFIG_VIRTIO_PMEM for booting from pmem`
- Link: https://github.com/kata-containers/kata-containers/commit/fbad186abec20279af94c5d85aab0ef20d6cf997

该提交说明：

```text
To support booting from pmem with cloud-hypervisor,
we need to enable the virtio-pmem in our kernel.
```

这说明 Cloud Hypervisor 的 Guest image PMEM 路径历史上明确使用 virtio-pmem。

---

## 4.2 2025 年 6 月：为 Cloud Hypervisor 增加可选切换能力

2025 年 6 月，Kata 增加了 Cloud Hypervisor 对 `disable_image_nvdimm` 的支持：

- Commit: `1aeef52baea18872ad6ea327dbb3aaba14d35fc7`
- Title: `clh: runtime: add disable_image_nvdimm support`
- Link: https://github.com/kata-containers/kata-containers/commit/1aeef52baea18872ad6ea327dbb3aaba14d35fc7

对应合并提交：

- Commit: `6f0ea595b7e79d7a72172422782ad1a839fdbd22`
- Title: `runtime: build variable for disable_image_nvdimm=true`
- Link: https://github.com/kata-containers/kata-containers/commit/6f0ea595b7e79d7a72172422782ad1a839fdbd22

对应 PR：

- PR: `#11402`
- Link: https://github.com/kata-containers/kata-containers/pull/11402

PR 描述明确表示：

```text
When using CLH, use virtio-blk instead of nvdimm for the rootfs device
if disable_image_nvdimm=true. QEMU already had this behavior.
```

但该阶段的提交说明同时明确写着：

```text
disable_image_nvdimm=false is the default config value.
```

所以 2025 年 6 月的变化是：

> Cloud Hypervisor 获得了“可选择禁用 image PMEM、改用 virtio-blk”的能力，但当时尚未全局默认禁用。

---

## 4.3 Kata 3.26.0：Cloud Hypervisor 默认仍允许 PMEM

在 Kata 3.26.0 的 `src/runtime/Makefile` 中，可以看到：

```makefile
DEFDISABLEIMAGENVDIMM ?= false
DEFDISABLEIMAGENVDIMM_NV = true
```

当时没有 Cloud Hypervisor 专用的：

```makefile
DEFDISABLEIMAGENVDIMM_CLH
```

因此 Cloud Hypervisor 继承通用默认值：

```text
disable_image_nvdimm = false
```

此外，3.26.0 的 `cloudHypervisor.setConfig()` 只复制配置：

```go
func (clh *cloudHypervisor) setConfig(config *HypervisorConfig) error {
    clh.config = *config
    return nil
}
```

没有强制关闭 PMEM。

---

## 4.4 Kata 3.27.0：Cloud Hypervisor 默认且强制禁用 image PMEM

Kata 3.27.0 中增加：

```makefile
DEFDISABLEIMAGENVDIMM_CLH ?= true
```

同时 Cloud Hypervisor backend 的 `setConfig()` 变为：

```go
func (clh *cloudHypervisor) setConfig(config *HypervisorConfig) error {
    clh.config = *config

    // We don't support NVDIMM with Cloud Hypervisor.
    clh.config.DisableImageNvdimm = true

    return nil
}
```

这意味着：

- 构建出的 Cloud Hypervisor 配置默认是 `disable_image_nvdimm=true`；
- 即使用户手工配置为 `false`，Go runtime 仍会在 backend 初始化时覆盖为 `true`；
- Guest image 的 virtio-pmem 路径实际上被强制关闭。

版本对比链接：

- `3.26.0...3.27.0`
- https://github.com/kata-containers/kata-containers/compare/3.26.0...3.27.0

---

## 4.5 2026 年 2 月：测试代码确认 CLH 已默认禁用

后续测试清理提交明确说明 Cloud Hypervisor 已默认使用 `disable_image_nvdimm=true`：

- Commit: `336b922d4f7fe5dc94087de2af3992a1a786d0d9`
- Title: `tests/cbl-mariner: Stop disabling NVDIMM explicitly`
- Link: https://github.com/kata-containers/kata-containers/commit/336b922d4f7fe5dc94087de2af3992a1a786d0d9

提交说明：

```text
This is not needed anymore since now disable_image_nvdimm=true
for Cloud Hypervisor.
```

该提交不是最初的功能切换提交，而是后续证据，说明测试不再需要通过 Pod annotation 单独设置该参数。

---

## 5. Cloud Hypervisor 的源码分支

Cloud Hypervisor 创建 Guest VM 时会判断：

```go
disableNvdimm := clh.config.DisableImageNvdimm || clh.config.ConfidentialGuest
enableDax := !disableNvdimm
```

随后对 Guest image 分支处理。

### 5.1 PMEM 分支

当 `DisableImageNvdimm=false` 时：

```go
pmem := chclient.NewPmemConfig(assetPath)
*pmem.DiscardWrites = true
pmem.SetIommu(clh.config.IOMMU)
clh.vmconfig.Pmem = ...
```

完整链路：

```text
Host kata-containers.img
→ Cloud Hypervisor PmemConfig
→ virtio-pmem
→ Guest /dev/pmem0
→ PMEM/DAX rootfs
```

### 5.2 virtio-blk 分支

当 `DisableImageNvdimm=true` 时：

```go
disk := chclient.NewDiskConfig()
disk.Path = &assetPath
disk.SetReadonly(true)
clh.vmconfig.Disks = ...
```

完整链路：

```text
Host kata-containers.img
→ Cloud Hypervisor DiskConfig
→ readonly=true
→ virtio-blk
→ Guest /dev/vda
→ Guest rootfs
```

### 5.3 3.27.0 的实际结果

由于 `setConfig()` 强制：

```go
clh.config.DisableImageNvdimm = true
```

因此 3.27.0 的 Cloud Hypervisor Guest image 必然进入只读 virtio-blk 分支，不会进入 `PmemConfig` 分支。

---

## 6. 是否因为 CVE-2026-24834

## 6.1 已确认的事实

源码可以确认：

1. 3.26.0 的 CLH image PMEM 路径默认开启。
2. 3.27.0 的 CLH image PMEM 路径被强制关闭。
3. 新路径明确使用 `DiskConfig.Readonly=true`。
4. 该变化切断了 Guest 原始 PMEM 设备直接对应共享 Host image 的路径。

## 6.2 与漏洞攻击面的对应关系

如果多个 Kata sandbox 共享同一个基础 image 文件，旧路径可能形成：

```text
Host 共享 kata-containers.img
→ virtio-pmem 映射进 Guest
→ Guest /dev/pmem0
→ Guest 内 rootfs 以 ro 挂载
→ 特权容器创建或访问原始 PMEM 设备节点
→ 绕过文件系统 ro mount
→ 直接修改块设备或 PMEM backing
→ 影响其他 sandbox 或后续启动
```

仅仅执行：

```text
mount -o ro
```

只限制该次文件系统 mount 的写行为，不等于 Host backing image 在 Hypervisor 层不可写。

切换到：

```go
disk.SetReadonly(true)
```

后，只读限制位于虚拟块设备层，攻击者即使绕过文件系统 mount 访问原始块设备，也应受到只读块设备属性约束。

## 6.3 尚未公开确认的部分

本次调查尚未找到明确写有以下内容的公开 commit 或 PR：

```text
Fixes CVE-2026-24834
```

因此以下内容属于基于源码和版本时间的高可信推断：

> Kata 3.27.0 强制 Cloud Hypervisor 使用只读 virtio-blk，是针对该 PMEM Guest image 写入风险的修复或主要缓解措施。

不应写成：

> 已找到官方公开 commit 明确声明该变更就是 CVE-2026-24834 修复。

---

## 7. 为什么 QEMU 仍能使用 NVDIMM

## 7.1 QEMU NVDIMM 具备只读 memory backend 支持

Kata 历史上专门增加了 QEMU 只读 NVDIMM 支持：

- Commit: `0d21263a9b55b062360aa82219a4e8611b548587`
- Title: `qemu: support read-only nvdimm`
- Link: https://github.com/kata-containers/kata-containers/commit/0d21263a9b55b062360aa82219a4e8611b548587

提交说明：

```text
Append readonly=on to a memory-backend-file object and
unarmed=on to a nvdimm device when ReadOnly is set to true.
```

对应合并提交：

- Commit: `0173713ea9128bb4c1804e1ad382d8e61f6309f8`
- Link: https://github.com/kata-containers/kata-containers/commit/0173713ea9128bb4c1804e1ad382d8e61f6309f8

因此 QEMU 可以生成类似：

```text
-object memory-backend-file,...,readonly=on
-device nvdimm,...,unarmed=on
```

这表示只读保护位于 Host memory backend 和 QEMU NVDIMM 设备模型层。

## 7.2 QEMU 对只读 backing file 的支持历史

Kata 还曾为 QEMU 打补丁或回移上游能力，使只读文件可以作为 NVDIMM memory backend：

- Commit: `1239ad0ba329e2237f46418793855059d8b38674`
- Title: `qemu: add kata patches for QEMU 5`
- Link: https://github.com/kata-containers/kata-containers/commit/1239ad0ba329e2237f46418793855059d8b38674

- Commit: `3f39df0d18ca93926190991689a6537e7bb67079`
- Title: `qemu: Add nvdimm read-only file support`
- Link: https://github.com/kata-containers/kata-containers/commit/3f39df0d18ca93926190991689a6537e7bb67079

- Merge commit: `9e6f1f7794729aa0be6775cf28898dea5a069ac9`
- Link: https://github.com/kata-containers/kata-containers/commit/9e6f1f7794729aa0be6775cf28898dea5a069ac9

这说明 QEMU 保留 NVDIMM 并非偶然，而是历史上专门补充过只读 backing 支持。

## 7.3 QEMU NVDIMM 与 CLH `discard_writes` 的区别

Cloud Hypervisor 旧 PMEM 分支设置：

```go
*pmem.DiscardWrites = true
```

但这不应直接等同于 QEMU 的：

```text
readonly=on
unarmed=on
```

两者的保护层次不同：

| 路径 | 保护机制 |
|---|---|
| Cloud Hypervisor virtio-pmem | `discard_writes=true` |
| QEMU NVDIMM | Host memory backend `readonly=on` + device `unarmed=on` |
| Cloud Hypervisor virtio-blk | `DiskConfig.Readonly=true` |

对于 PMEM/DAX，Guest 数据访问可能通过内存映射和 CPU load/store 完成，而不是所有写入都经过普通 virtqueue block write 请求。因此，将共享 Guest image 改成明确的只读 virtio-blk，是更容易验证和约束的安全路径。

目前尚未从 Cloud Hypervisor 上游源码完全证明 `discard_writes` 在所有 DAX 写路径上的具体缺陷，不能仅凭字段名称断言它一定无效。但从 Kata 的最终修复选择看，上游不再依赖该路径保护共享 Guest image。

---

## 8. 为什么不是把 QEMU NVDIMM 也全部禁用

可能原因包括：

1. QEMU 已具备经过 Kata 集成的只读 NVDIMM backend。
2. QEMU 能在 Host backing 层设置 `readonly=on`。
3. QEMU 能在 NVDIMM 设备上设置 `unarmed=on`。
4. Kata 历史上专门维护和回移过只读 NVDIMM 支持。
5. NVDIMM+DAX 对只读 Guest image 的启动和读取性能仍可能有价值。
6. 漏洞核心不是“所有 PMEM 都不安全”，而是共享 Host backing 是否能被 Guest 修改。

因此更准确的安全结论是：

```text
不安全的不是 PMEM 名称本身，
而是 Host backing 的只读边界是否在 Hypervisor 层真正成立。
```

---

## 9. 配置项边界

### 9.1 `disable_image_nvdimm`

控制对象：

```text
Kata Guest 基础 image
```

即：

```text
/opt/kata/share/kata-containers/kata-containers.img
→ Guest VM rootfs
```

它不直接控制容器 rootfs 或 Kubernetes 数据卷。

### 9.2 `block_device_driver`

控制对象：

```text
容器 rootfs 为块设备时的设备传输方式
或其他直接附加块设备
```

可能值包括：

```text
virtio-blk
virtio-scsi
nvdimm
```

该配置与 Guest 基础 image 的 `disable_image_nvdimm` 不是同一个概念。

### 9.3 `disable_block_device_use`

控制对象：

```text
是否允许把 containerd snapshotter 提供的容器 rootfs 块设备直接传入 Guest
```

如果禁用，容器 rootfs 通常改走 virtio-fs 共享目录。

---

## 10. 对实际环境的影响

以 Kata 3.27.0 + Cloud Hypervisor 为例：

```text
Guest OS rootfs：
kata-containers.img
→ readonly virtio-blk
→ Guest /dev/vda

Container rootfs：
Host snapshot/rootfs
→ 通常由 virtio-fs 共享
→ Kata Agent
→ Container mount namespace /
```

所以容器中执行：

```bash
findmnt /
```

看到 `virtiofs`，通常表示容器 rootfs 使用 virtio-fs，不代表 Guest OS rootfs 也是 virtio-fs。

同一台 Kata VM 中可能同时存在：

```text
Guest rootfs      → readonly virtio-blk
Container rootfs  → virtio-fs
PVC               → virtio-blk 或其他块设备
emptyDir          → virtio-fs 或 tmpfs
/dev/shm          → tmpfs
```

---

## 11. 验证命令

### 11.1 确认 Kata 版本

```bash
kata-runtime --version
containerd-shim-kata-v2 --version
cloud-hypervisor --version
qemu-system-aarch64 --version
```

### 11.2 查看生效配置

```bash
grep -RniE \
'disable_image_nvdimm|block_device_driver|vm_rootfs_driver|disable_block_device_use|shared_fs' \
/opt/kata/share/defaults/kata-containers \
/etc/kata-containers 2>/dev/null
```

### 11.3 Cloud Hypervisor Host 侧

```bash
ps -ef | grep '[c]loud-hypervisor'
journalctl -u containerd | grep -Ei 'pmem|nvdimm|virtio-blk|disable_image_nvdimm'
```

如果使用 CLH API config，可重点确认：

```text
pmem: [...]
```

还是：

```text
disks: [...]
readonly: true
```

### 11.4 QEMU Host 侧

```bash
ps -ef | grep '[q]emu' | grep -E 'memory-backend-file|nvdimm|virtio-blk'
```

重点检查：

```text
readonly=on
unarmed=on
```

### 11.5 Guest 侧

```bash
cat /proc/cmdline
findmnt /
lsblk -o NAME,TYPE,RO,FSTYPE,MOUNTPOINTS
cat /sys/block/pmem0/ro 2>/dev/null
cat /sys/block/vda/ro 2>/dev/null
blockdev --getro /dev/pmem0 2>/dev/null
blockdev --getro /dev/vda 2>/dev/null
dmesg | grep -Ei 'virtio.?pmem|nvdimm|pmem|virtio.?blk'
```

---

## 12. 关键提交与 PR 汇总

| 类型 | SHA / 编号 | 内容 | 链接 |
|---|---|---|---|
| Commit | `fbad186abec20279af94c5d85aab0ef20d6cf997` | 为 Cloud Hypervisor PMEM 启用 `CONFIG_VIRTIO_PMEM` | https://github.com/kata-containers/kata-containers/commit/fbad186abec20279af94c5d85aab0ef20d6cf997 |
| Commit | `1aeef52baea18872ad6ea327dbb3aaba14d35fc7` | CLH 增加 `disable_image_nvdimm` 支持 | https://github.com/kata-containers/kata-containers/commit/1aeef52baea18872ad6ea327dbb3aaba14d35fc7 |
| PR | `#11402` | CLH 在禁用 image NVDIMM 时改用 virtio-blk | https://github.com/kata-containers/kata-containers/pull/11402 |
| Merge commit | `6f0ea595b7e79d7a72172422782ad1a839fdbd22` | 合并 `disable_image_nvdimm` 构建变量及 CLH 支持 | https://github.com/kata-containers/kata-containers/commit/6f0ea595b7e79d7a72172422782ad1a839fdbd22 |
| Commit | `336b922d4f7fe5dc94087de2af3992a1a786d0d9` | 测试确认 CLH 已默认 `disable_image_nvdimm=true` | https://github.com/kata-containers/kata-containers/commit/336b922d4f7fe5dc94087de2af3992a1a786d0d9 |
| Commit | `0d21263a9b55b062360aa82219a4e8611b548587` | QEMU 增加只读 NVDIMM 参数 | https://github.com/kata-containers/kata-containers/commit/0d21263a9b55b062360aa82219a4e8611b548587 |
| Merge commit | `0173713ea9128bb4c1804e1ad382d8e61f6309f8` | 合并 QEMU 只读 NVDIMM 支持 | https://github.com/kata-containers/kata-containers/commit/0173713ea9128bb4c1804e1ad382d8e61f6309f8 |
| Commit | `1239ad0ba329e2237f46418793855059d8b38674` | 为 QEMU 5 加入只读 NVDIMM backing patch | https://github.com/kata-containers/kata-containers/commit/1239ad0ba329e2237f46418793855059d8b38674 |
| Commit | `3f39df0d18ca93926190991689a6537e7bb67079` | 回移 QEMU NVDIMM 只读文件支持 | https://github.com/kata-containers/kata-containers/commit/3f39df0d18ca93926190991689a6537e7bb67079 |
| Merge commit | `9e6f1f7794729aa0be6775cf28898dea5a069ac9` | 合并 QEMU NVDIMM 只读文件支持 | https://github.com/kata-containers/kata-containers/commit/9e6f1f7794729aa0be6775cf28898dea5a069ac9 |
| Commit | `ece5edc641330c09146c5f72c43f73f2724e1574` | ARM64 无 UEFI/ACPI 时禁用 QEMU image NVDIMM | https://github.com/kata-containers/kata-containers/commit/ece5edc641330c09146c5f72c43f73f2724e1574 |
| Compare | `3.26.0...3.27.0` | 版本差异 | https://github.com/kata-containers/kata-containers/compare/3.26.0...3.27.0 |

---

## 13. 已确认内容与待确认内容

### 已确认

- Cloud Hypervisor 历史上使用 virtio-pmem 承载 Guest image。
- 2025 年 6 月 Kata 为 CLH 增加可选的 `disable_image_nvdimm` 切换。
- Kata 3.26.0 默认仍为 `disable_image_nvdimm=false`。
- Kata 3.27.0 将 CLH 默认改为 `true`，并在 Go runtime 中强制设为 `true`。
- CLH 禁用 PMEM 后，Guest image 使用只读 virtio-blk。
- QEMU 已实现 `readonly=on + unarmed=on` 的只读 NVDIMM。
- QEMU 历史上专门维护过只读 NVDIMM backing file 支持。

### 推断

- 3.27.0 的 CLH 强制切换是 CVE-2026-24834 的修复或主要缓解。
- 选择 virtio-blk 与 CLH PMEM/DAX 写保护语义有关。

### 尚未从公开源码确认

- 哪个具体公开 commit 被官方直接标记为 CVE-2026-24834 的修复提交。
- Cloud Hypervisor `discard_writes=true` 在该漏洞中的确切失效点。
- 该漏洞是否只影响特定 Cloud Hypervisor 版本、架构或 image 构建格式。
- QEMU NVDIMM 在所有 Kata 版本和所有启动参数组合下是否均安全；仍需核对实际生成的 `readonly=on` 和 `unarmed=on`。

---

## 14. 最终总结

Kata 对 Cloud Hypervisor 的变化可以概括为：

```text
Kata 3.26.0：
Guest image
→ Cloud Hypervisor PmemConfig
→ virtio-pmem
→ /dev/pmem0
→ DAX/PMEM rootfs

Kata 3.27.0：
Guest image
→ DisableImageNvdimm 强制 true
→ readonly DiskConfig
→ virtio-blk
→ /dev/vda
→ 只读 Guest rootfs
```

QEMU 没有同步彻底禁用 NVDIMM，是因为其实现具备：

```text
Host memory backend readonly=on
+ NVDIMM unarmed=on
```

所以安全判断不能简化为：

```text
virtio-pmem 不安全，NVDIMM 安全
```

更准确的是：

```text
安全性取决于共享 Host backing 是否在 Hypervisor 层真正只读，
而不是 Guest 文件系统是否仅以 ro 方式挂载。
```
