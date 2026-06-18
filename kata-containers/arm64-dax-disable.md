# Kata Containers：ARM64 禁用 DAX 相关源码分析

## 1. 结论

Kata Containers 仓库中确实有 **ARM / aarch64 禁用 DAX** 的源码逻辑。

但这里需要注意：

```text
这里主要禁用的是 Guest VM rootfs / PMEM / NVDIMM 路径上的 DAX，
不是简单地说所有 DAX 都不能用。
```

核心结论：

```text
ARM64 下：
Guest image 可以通过 NVDIMM / PMEM 设备暴露给 Guest VM；
但 Kata 源码会禁用 rootfs DAX；
因此 Guest rootfs 挂载参数不会带 dax。
```

也就是说，在 ARM64 / 鲲鹏环境里，不建议把这个方案直接表述为：

```text
Guest rootfs = PMEM + DAX
```

更准确的表述应该是：

```text
Guest rootfs = PMEM / NVDIMM 设备方式暴露，
但 ARM64 下 Kata 禁用了 rootfs DAX。
```

---

## 2. 两类 DAX 要分清楚

Kata 中容易混淆的 DAX 主要有两类：

| 类型 | 相关配置 / 代码 | 是否看到 ARM 特判 | 说明 |
|---|---|---|---|
| Guest VM rootfs / PMEM / NVDIMM DAX | `qemu_arm64.go`、`runtime-rs qemu cmdline`、`rootflags=dax` | 是 | ARM64 明确禁用 |
| virtio-fs DAX cache | `virtio_fs_cache_size` | 未看到 ARM 专属特判 | 默认值为 0，默认不启用 |

所以源码中这句：

```text
DAX is disabled on ARM due to a kernel panic in caches_clean_inval_pou.
```

主要指的是：

```text
Guest image 作为 PMEM / NVDIMM rootfs 挂载时，
是否给 guest kernel 添加 rootflags=dax。
```

---

## 3. Go runtime：qemu_arm64.go 中禁用 DAX

源码位置：

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
    measurementAlgo: config.MeasurementAlgo,
}
```

这里可以看到：

```text
arm64 + qemu 路径下，qemuArchBase.dax 被设置为 false。
```

这表示：

```text
即使 Guest image 通过 NVDIMM / PMEM 方式传入，
ARM64 下也不会启用 rootfs DAX。
```

---

## 4. x86_64 对比：amd64 中 dax=true

源码位置：

```text
src/runtime/virtcontainers/qemu_amd64.go
```

关键逻辑：

```go
q := &qemuAmd64{
    qemuArchBase: qemuArchBase{
        qemuMachine:          *mp,
        qemuExePath:          defaultQemuPath,
        memoryOffset:         config.MemOffset,
        kernelParamsNonDebug: kernelParamsNonDebug,
        kernelParamsDebug:    kernelParamsDebug,
        kernelParams:         kernelParams,
        disableNvdimm:        config.DisableImageNvdimm,
        dax:                  true,
        protection:           noneProtection,
        legacySerial:         config.LegacySerial,
    },
}
```

对比关系：

```text
amd64:
    dax = true

arm64:
    dax = false
```

说明这是一个架构差异，不是配置文件偶然导致的。

---

## 5. 历史变化：arm64 以前也曾是 dax=true

相关提交前，`qemu_arm64.go` 中 arm64 也曾是：

```go
dax: true,
```

后来改成：

```go
// DAX is disabled on ARM due to a kernel panic in caches_clean_inval_pou.
dax: false,
```

这说明：

```text
ARM64 禁用 DAX 是 Kata 后续主动修改的行为，
不是本地环境或配置文件的问题。
```

---

## 6. runtime-rs 中也有同样逻辑

源码位置：

```text
src/runtime-rs/crates/hypervisor/src/qemu/cmdline_generator.rs
```

关键逻辑：

```rust
// DAX is disabled on ARM due to a kernel panic in caches_clean_inval_pou.
#[cfg(target_arch = "aarch64")]
let use_dax = false;

#[cfg(not(target_arch = "aarch64"))]
let use_dax = true;

let mut rootfs_params = KernelParams::new_rootfs_kernel_params(
    &config.boot_info.kernel_verity_params,
    &config.boot_info.vm_rootfs_driver,
    &config.boot_info.rootfs_type,
    use_dax,
)?;
```

这说明：

```text
Go runtime 和 Rust runtime 都对 ARM64 做了禁用 DAX 的处理。
```

也就是说，即使后续看 Kata 4.0 / runtime-rs，这个逻辑仍然存在。

---

## 7. use_dax 具体影响什么？

源码位置：

```text
src/runtime-rs/crates/hypervisor/src/kernel_param.rs
```

核心逻辑是：如果 rootfs driver 是 PMEM，则根据 `use_dax` 决定是否给 kernel rootflags 加 `dax`。

简化后逻辑如下：

```rust
VM_ROOTFS_DRIVER_PMEM => {
    params.push(Param::new("root", VM_ROOTFS_ROOT_PMEM));

    match rootfs_type {
        VM_ROOTFS_FILESYSTEM_EXT4 => {
            if use_dax {
                rootflags = "dax,data=ordered,errors=remount-ro ro";
            } else {
                rootflags = "data=ordered,errors=remount-ro ro";
            }
        }
        VM_ROOTFS_FILESYSTEM_XFS | VM_ROOTFS_FILESYSTEM_EROFS => {
            if use_dax {
                rootflags = "dax ro";
            } else {
                rootflags = "ro";
            }
        }
    }
}
```

所以 ARM64 下的实际效果是：

```text
root=/dev/pmem0p1
rootfstype=ext4 / xfs / erofs
rootflags 不带 dax
```

而不是：

```text
rootflags=dax,...
```

---

## 8. 文档中 DAX 的设计意图

Kata 架构文档中描述的理想链路是：

```text
Host kata-containers.img
        |
        | hypervisor DAX / NVDIMM / PMEM
        v
Guest VM /dev/pmem0p1
        |
        v
Guest VM rootfs
```

也就是说，Kata 设计上支持通过 DAX 将 guest image 共享进 VM，并作为 VM rootfs 使用。

但是 ARM64 源码里对这个行为做了限制：

```text
arm64:
    可以使用 PMEM / NVDIMM 设备暴露 Guest image；
    但是 dax=false；
    guest kernel rootflags 不带 dax。
```

---

## 9. virtio-fs DAX cache 的区别

配置文件里还有一个参数：

```toml
virtio_fs_cache_size = @DEFVIRTIOFSCACHESIZE@
```

注释是：

```text
Default size of DAX cache in MiB
```

Makefile 中默认值为：

```makefile
# Default DAX mapping cache size in MiB
#if value is 0, DAX is not enabled
DEFVIRTIOFSCACHESIZE ?= 0
```

也就是说：

```text
virtio-fs DAX cache 默认就是 0，默认不启用。
```

但是这里要注意：

```text
virtio_fs_cache_size = 0 是通用默认值；
源码中明确 ARM 特判的是 Guest rootfs / PMEM / NVDIMM DAX。
```

不要把这两个问题混为一谈。

---

## 10. 结合当前 ARM64 / 鲲鹏环境的理解

如果在容器内看到：

```bash
findmnt -T /
# / none virtiofs
```

说明：

```text
容器 rootfs 走 virtio-fs。
```

如果在 Guest / 容器内能看到：

```bash
/dev/pmem0
/dev/pmem0p1
```

说明：

```text
Guest image / VM rootfs 可能通过 PMEM / NVDIMM 设备形式暴露进来了。
```

但是在 ARM64 下，Kata 源码已经把 DAX 关掉了，所以更准确的表述是：

```text
Guest image 通过 PMEM / NVDIMM 设备暴露，
但 ARM64 下 rootfs DAX 被禁用。
```

而不是：

```text
Guest rootfs = PMEM + DAX
```

---

## 11. 对性能测试和方案判断的影响

如果当前方案目标是：

```text
容器 rootfs = virtio-fs
Guest VM rootfs = PMEM / NVDIMM
希望依赖 DAX 提升性能
```

那么在 ARM64 上需要谨慎，因为源码层面已经禁用了 rootfs DAX。

实际性能路径应拆成两部分看：

```text
1. 容器 rootfs 路径
   Host shared dir -> virtiofsd -> virtio-fs -> Guest -> Container /

2. Guest VM rootfs 路径
   Host kata-containers.img -> QEMU NVDIMM / PMEM -> Guest /dev/pmem0p1
   ARM64 下不带 dax rootflags
```

所以如果业务主要访问的是容器 rootfs 或共享 volume：

```text
瓶颈更可能在 virtio-fs 路径。
```

如果测试的是 Guest rootfs 里的 PMEM 分区：

```text
ARM64 下也不能简单认为测到的是 PMEM+DAX 性能。
```

---

## 12. 汇报口径

可以这样说：

> 我查了 Kata 源码，ARM64 下 QEMU 路径明确把 `dax` 设置为 `false`，注释原因是 ARM 上 DAX 会触发 `caches_clean_inval_pou` 相关 kernel panic。Rust runtime 里也有同样逻辑：`target_arch = "aarch64"` 时 `use_dax = false`。所以在 ARM64 上，PMEM/NVDIMM 可以作为 Guest VM rootfs 设备暴露，但 rootfs 挂载参数不会带 `dax`。另外，virtio-fs 的 DAX cache 由 `virtio_fs_cache_size` 控制，默认值是 0，表示不启用 DAX cache。

更短一点：

```text
Kata 代码层面已经对 ARM64 禁用了 rootfs DAX：
Go runtime: qemu_arm64.go -> dax=false
Rust runtime: target_arch=aarch64 -> use_dax=false
原因：避免 caches_clean_inval_pou kernel panic。
因此 ARM64 上不要把这个方案表述为真正的 PMEM+DAX rootfs。
```

---

## 13. 关键源码位置

```text
src/runtime/virtcontainers/qemu_arm64.go
    ARM64 QEMU 路径，dax=false

src/runtime/virtcontainers/qemu_amd64.go
    AMD64 QEMU 路径，dax=true

src/runtime-rs/crates/hypervisor/src/qemu/cmdline_generator.rs
    Rust runtime 中 aarch64 use_dax=false

src/runtime-rs/crates/hypervisor/src/kernel_param.rs
    根据 use_dax 生成 rootflags，决定是否带 dax

src/runtime/config/configuration-qemu.toml.in
    virtio_fs_cache_size 配置说明

src/runtime/Makefile
    DEFVIRTIOFSCACHESIZE 默认值为 0
```
