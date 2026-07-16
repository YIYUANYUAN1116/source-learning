# Kata Containers ARM64 DAX 问题调查

## 1. 调查背景

在 Kata Containers 的 ARM64 QEMU 实现中，当前源码存在如下配置：

```go
// DAX is disabled on ARM due to a kernel panic in caches_clean_inval_pou.
dax: false,
```

该配置意味着：即使 Kata Guest image 通过 QEMU NVDIMM 暴露为 Guest 内的 PMEM 设备，Kata 默认也不会为 ARM64 Guest rootfs 自动添加 `rootflags=dax`。

本次调查主要回答以下问题：

1. `caches_clean_inval_pou` 崩溃具体发生在什么路径。
2. 为什么 ext4 DAX 会触发 ARM64 cache maintenance。
3. 该问题与之前的 `dax_disassociate_entry` 崩溃有什么区别。
4. 强制通过 `kernel_params` 开启 DAX 可能产生什么影响。
5. 当前环境应该如何验证和使用。

## 2. 结论

该问题发生在以下组合中：

```text
ARM64 Guest
+ QEMU NVDIMM
+ Guest image 作为 /dev/pmem0p1
+ ext4 rootfs
+ rootflags=dax
```

Guest 启动后，`/sbin/init` 或动态链接库等可执行文件位于 ext4 DAX 文件系统中。当进程首次访问或执行这些文件时，会触发 DAX page fault。Linux 将 PMEM page 直接插入用户态页表，并在 ARM64 上执行指令缓存和数据缓存一致性维护。

实际调用链如下：

```text
用户进程访问 DAX 文件
→ ext4_dax_fault
→ ext4_dax_huge_fault
→ dax_iomap_fault
→ dax_iomap_pte_fault
→ vmf_insert_page_mkwrite
→ insert_page
→ insert_page_into_pte_locked
→ __sync_icache_dcache
→ sync_icache_aliases
→ caches_clean_inval_pou
→ synchronous external abort
→ PID 1 崩溃
→ Guest kernel panic
```

Kata 因此在 ARM64 上默认关闭 DAX，以避免 Guest 启动阶段直接崩溃。

需要注意：关闭的是 DAX，不是 NVDIMM。关闭 DAX 后，Guest image 仍可通过 QEMU NVDIMM 作为 `/dev/pmem0p1` 使用，只是 ext4 恢复传统 page cache 路径。

## 3. Kata 源码位置

仓库：

```text
kata-containers/kata-containers
```

文件：

```text
src/runtime/virtcontainers/qemu_arm64.go
```

关键逻辑：

```go
q := &qemuArm64{
    qemuArchBase: qemuArchBase{
        qemuMachine:          supportedQemuMachine,
        qemuExePath:          defaultQemuPath,
        memoryOffset:         config.MemOffset,
        kernelParamsNonDebug: kernelParamsNonDebug,
        kernelParamsDebug:    kernelParamsDebug,
        kernelParams:         kernelParams,
        disableNvdimm:        config.DisableImageNvdimm,
        // DAX is disabled on ARM due to a kernel panic in caches_clean_inval_pou.
        dax:          false,
        protection:   noneProtection,
        legacySerial: config.LegacySerial,
    },
}
```

该 `dax` 字段会继续传递到 Guest rootfs kernel parameter 生成逻辑。

对于 ext4：

```go
if dax {
    rootflags = "dax,data=ordered,errors=remount-ro ro"
} else {
    rootflags = "data=ordered,errors=remount-ro ro"
}
```

因此 ARM64 默认生成：

```text
rootflags=data=ordered,errors=remount-ro ro
```

而不是：

```text
rootflags=dax,data=ordered,errors=remount-ro ro
```

## 4. 问题触发条件

### 4.1 QEMU 将 Guest image 暴露为 NVDIMM

当配置：

```toml
disable_image_nvdimm = false
```

ARM64 QEMU 路径会调用 `appendNvdimmImage()`，最终形成：

```text
Host kata-containers.img
→ QEMU memory-backend-file
→ QEMU NVDIMM
→ Guest /dev/pmem0
→ Guest /dev/pmem0p1
```

### 4.2 Guest rootfs 使用 ext4 DAX

如果 kernel cmdline 包含：

```text
rootflags=dax,data=ordered,errors=remount-ro
```

Linux 会把该字符串作为根文件系统挂载参数传给 ext4。其中 `dax` 是旧式 DAX 挂载参数，等价于 `dax=always`。

### 4.3 执行 Guest rootfs 中的程序

Guest 启动 `/sbin/init` 时，会访问可执行文件、动态链接器、libc 和共享库，这些文件都位于 DAX rootfs 上，因此会触发 DAX page fault。

## 5. 为什么会进入 caches_clean_inval_pou

ARM64 采用分离的指令缓存和数据缓存。当文件页将映射为用户态可执行页面时，Linux 需要保证 I-cache 与 D-cache 一致。

相关路径为：

```text
__sync_icache_dcache
→ sync_icache_aliases
→ caches_clean_inval_pou
```

`caches_clean_inval_pou` 的作用是清理数据缓存并使指令缓存失效，保证指定区域的 I-cache 与 D-cache 一致。

普通 page cache 场景中，这些地址通常对应普通 Guest RAM。DAX 场景中，文件页直接对应 PMEM/NVDIMM page frame。当内核对这些 PMEM 映射执行 cache maintenance 时，QEMU/KVM/Guest kernel/平台组合触发了 external abort。

## 6. 实际崩溃现象

Kata 提交记录表明，Guest 在崩溃前已经完成：

```text
EXT4-fs (pmem0p1): mounted filesystem
VFS: Mounted root readonly on device 259:1
Run /sbin/init as init process
```

随后出现：

```text
Internal error: synchronous external abort
Tainted: [M]=MACHINE_CHECK
pc : caches_clean_inval_pou
lr : sync_icache_aliases
```

调用链包括：

```text
caches_clean_inval_pou
__sync_icache_dcache
insert_page_into_pte_locked
insert_page
vmf_insert_page_mkwrite
dax_iomap_pte_fault
dax_iomap_fault
ext4_dax_huge_fault
ext4_dax_fault
handle_mm_fault
```

这说明问题不是 PMEM 设备不存在或 ext4 挂载失败，而是 DAX 文件页映射和 ARM64 cache maintenance 阶段发生异常。

## 7. synchronous external abort 的含义

ARM64 的 synchronous external abort 表示 CPU 正在执行同步内存访问或 cache maintenance 指令时，底层内存系统或虚拟化平台返回了不可恢复的外部异常。

当前可以确认：

```text
异常位置：caches_clean_inval_pou
操作对象：DAX fault 插入的 PMEM page
异常类型：synchronous external abort / machine check
结果：Guest kernel oops 或 panic
```

从 Kata 提交本身尚不能确认最终责任属于哪一层：

```text
Linux ARM64 DAX/ZONE_DEVICE 处理
QEMU NVDIMM 内存属性
KVM 对 cache maintenance 的处理
Guest firmware/ACPI NVDIMM 描述
特定 ARM CPU 或平台限制
```

因此，精确根因仍应标记为尚未完全确认。

## 8. 与 dax_disassociate_entry 问题的区别

ARM64 DAX 曾出现过另一个崩溃：

```text
dax_disassociate_entry
→ dax_insert_entry
→ dax_iomap_pte_fault
→ ext4_dax_fault
```

其错误类型主要是：

```text
Unable to handle kernel paging request
level 2 translation fault
```

该问题发生在 DAX entry/page bookkeeping 路径。

后续 Kata 提交说明，相关 Linux patch 已获批准，因此重新启用了 DAX。但完整 QEMU NVDIMM 支持启用后，又出现新的 `caches_clean_inval_pou` 崩溃。

| 对比项 | 早期问题 | 当前问题 |
|---|---|---|
| 崩溃函数 | dax_disassociate_entry | caches_clean_inval_pou |
| 所在层次 | DAX entry/page 管理 | ARM64 cache maintenance |
| 异常表现 | translation fault | synchronous external abort |
| 是否同一问题 | 否 | 否 |
| Kata 处理 | 临时关闭后重新启用 | 再次关闭 ARM64 DAX |

## 9. Kata 的规避方案

Kata 当前 ARM64 QEMU 实现设置：

```go
dax: false
```

默认行为变为：

```text
Host kata-containers.img
→ QEMU NVDIMM
→ Guest /dev/pmem0p1
→ ext4
→ 传统 Guest page cache
```

不会自动进入：

```text
ext4 DAX
→ PMEM page 直接映射
→ ARM64 caches_clean_inval_pou
```

该方案保留了 NVDIMM/PMEM 设备路径，但牺牲了 DAX 绕过 page cache 的性能收益。

## 10. 手工覆盖 rootflags=dax 的影响

如果配置：

```toml
kernel_params = "cgroup_no_v1=all systemd.unified_cgroup_hierarchy=1 rootflags=dax,data=ordered,errors=remount-ro"
```

用户参数会被追加到 Guest kernel cmdline 后部，从而覆盖 Kata 自动生成的无 DAX `rootflags`。这相当于绕过 Kata ARM64 默认保护，强制 ext4 rootfs 开启 DAX。

可能影响包括：

### 10.1 Guest 启动失败

```text
/sbin/init page fault
→ ext4 DAX fault
→ caches_clean_inval_pou
→ external abort
→ PID 1 崩溃
→ Guest kernel panic
→ Kata Pod 创建失败
```

### 10.2 运行期进程崩溃

即使 Guest 能启动，执行新程序、加载动态库、`mmap(PROT_EXEC)`、JIT 或首次访问 DAX 映射文件时仍可能触发。

### 10.3 缓存一致性风险

理论上可能出现执行旧指令、代码修改不可见、mmap 数据不一致或随机进程异常。Kata 已记录的是明确 external abort，尚未证实存在静默数据损坏。

### 10.4 生产稳定性风险

某次测试成功只能证明当前 CPU、固件、KVM、QEMU、Guest kernel 和负载组合暂未触发问题，不能据此认为 ARM64 DAX 已获得 Kata 上游正式支持。

## 11. 对鲲鹏 ARM64 环境的影响

当前测试环境与上游故障组合高度接近：

```text
ARM64 鲲鹏
QEMU 10.2.1
Guest kernel 6.18.x
Guest image NVDIMM
ext4 /dev/pmem0p1
```

因此，通过 `kernel_params` 强制添加 `rootflags=dax` 应视为实验性配置，而不是正式部署参数。

建议：

1. 仅在测试 RuntimeClass 中启用。
2. 不应用于默认 Kata RuntimeClass。
3. 只用于受控节点和受控 Pod。
4. 持续监控 Guest dmesg。
5. 验证进程执行、动态库加载和 mmap 场景。
6. 保留关闭 DAX 的快速回退配置。

## 12. 验证命令

检查 Guest kernel cmdline：

```bash
cat /proc/cmdline
```

检查 Guest rootfs：

```bash
findmnt -T / -o TARGET,SOURCE,FSTYPE,OPTIONS
```

检查容器 PMEM 挂载：

```bash
findmnt -T /pmem -o TARGET,SOURCE,FSTYPE,OPTIONS
```

检查 PMEM/NVDIMM/DAX 日志：

```bash
dmesg | grep -Ei 'dax|pmem|nvdimm|external abort|machine check|oops|panic|caches_clean_inval_pou'
```

进程执行压力测试：

```bash
for i in $(seq 1 10000); do
    /pmem/usr/bin/true || break
done
```

动态库加载测试：

```bash
for i in $(seq 1 1000); do
    /pmem/usr/bin/ldd /pmem/usr/bin/bash >/dev/null || break
done
```

持续观察 Guest kernel：

```bash
dmesg -w
```

## 13. 对应 Commit

### 13.1 当前 ARM64 DAX 再次关闭

Commit：

```text
48aa077e8cb41226598c3e9f3f46a72d2f57ee4d
runtime{,-rs}/qemu/arm64: Disable DAX
```

链接：

https://github.com/kata-containers/kata-containers/commit/48aa077e8cb41226598c3e9f3f46a72d2f57ee4d

该提交记录了 `caches_clean_inval_pou`、synchronous external abort、`ext4_dax_fault` 以及 PID 1 崩溃。

### 13.2 早期 ARM64 DAX 临时关闭

Commit：

```text
2acb94ef2d2c3e64b9c9e01160f85f64df76f323
arm64: Do not use DAX with the rootfs image
```

链接：

https://github.com/kata-containers/kata-containers/commit/2acb94ef2d2c3e64b9c9e01160f85f64df76f323

该提交对应另一个问题：`dax_disassociate_entry`、level 2 translation fault 和 Guest kernel panic。

### 13.3 早期问题修复后重新启用 DAX

Commit：

```text
33b1f0786e6532179478735398af5baf9f70f57a
Revert "arm64: Do not use DAX with the rootfs image"
```

链接：

https://github.com/kata-containers/kata-containers/commit/33b1f0786e6532179478735398af5baf9f70f57a

该提交说明早期 `dax_disassociate_entry` 问题已有获批 Linux patch，因此恢复 ARM64 DAX。但后续又出现 `caches_clean_inval_pou` 新问题，因此 commit `48aa077e...` 再次关闭 ARM64 DAX。

## 14. Commit 演进关系

```text
ARM64 DAX 默认开启
        │
        ▼
2acb94ef
发现 dax_disassociate_entry 崩溃
临时关闭 ARM64 DAX
        │
        ▼
33b1f078
相关 Linux patch 获批
重新启用 ARM64 DAX
        │
        ▼
48aa077e
完整 QEMU NVDIMM 下出现 caches_clean_inval_pou 崩溃
再次关闭 ARM64 DAX
```

## 15. 文档结论

Kata Containers 当前在 ARM64 QEMU 实现中默认关闭 Guest image rootfs 的 DAX。

其直接原因是：在完整 QEMU NVDIMM 模式下，ext4 DAX 文件发生 page fault 并直接映射 PMEM page 时，ARM64 Guest kernel 会进入 `__sync_icache_dcache` 和 `caches_clean_inval_pou` 执行指令缓存与数据缓存一致性维护。上游测试中，该 cache maintenance 操作触发 synchronous external abort，导致 PID 1 崩溃并引发 Guest kernel panic。

关闭 DAX 后，Guest image 仍可以通过 QEMU NVDIMM 暴露为 `/dev/pmem0p1`，但 ext4 使用传统 page cache 路径，从而绕开 DAX page fault 和 PMEM page cache-maintenance 路径。

手工通过 `kernel_params` 添加 `rootflags=dax` 会绕过 Kata 的 ARM64 默认保护逻辑。即使当前环境能够正常启动，也只能作为实验性配置使用，需要针对进程执行、动态库加载、mmap、并发文件访问和长时间稳定性进行专项验证，不应直接作为生产环境默认配置。

## 16. 已确认与待确认

### 已确认

- Kata 在 ARM64 QEMU 实现中明确设置 `dax: false`。
- 当前注释对应 commit 为 `48aa077e8cb41226598c3e9f3f46a72d2f57ee4d`。
- 故障发生在 ext4 DAX page fault 路径。
- 最终异常点为 ARM64 `caches_clean_inval_pou`。
- 异常类型为 synchronous external abort / machine check。
- PID 1 崩溃会导致 Guest kernel panic。
- 关闭 DAX 可以避开该路径。
- 该问题与早期 `dax_disassociate_entry` 问题不同。

### 待确认

- 最终责任组件是 Linux、QEMU、KVM、firmware 还是特定 ARM 平台。
- 是否已有专门针对 `caches_clean_inval_pou` 问题的上游 Linux 修复。
- 哪个 Guest kernel 版本可以安全重新启用 ARM64 DAX。
- 鲲鹏当前固件、KVM 和 QEMU 组合是否稳定复现相同 external abort。
