# 领域内子包依赖规范

本文档详述 goddd 架构中领域内部子包的拆分原则、依赖方向与循环依赖解决方案。

---

## 核心依赖原则

| 规则 | 说明 |
|------|------|
| **子包单向依赖根包** | 所有子包（如 `stores/`、`sub-a/`、`adapter/`）均可直接 import 领域根包（`internal/core/<domain>`），根包不得反向 import 子包 |
| **同级子包可直接依赖** | 领域内的同级子包之间若有单向依赖关系，可直接 import，无需过度抽象出接口 |
| **禁止循环依赖** | 当子包 A 与子包 B 出现双向依赖时，必须在其中一方定义狭窄接口（Narrow Interface）打破循环 |
| **外部基础设施接口在根包** | 外部 MQ、DB、三方系统驱动等依赖的 Port 接口统一定义在领域根包的 `port.go` 中 |

---

## 判断是否需要定义接口

```
需要接口的场景：
├── 1. 实现方位于领域外部（如 MQ 客户端、DB 驱动、跨领域的被动适配器）
├── 2. 领域内子包之间出现双向调用（通过定义 Narrow Interface 解除循环依赖）
└── 3. 单元测试需要 Mock 的外部 IO 边界

不需要接口的场景：
├── 1. 子包 A 单向调用子包 B（直接 import 并调用具体函数/结构体）
├── 2. 子包直接使用根包定义的领域模型（Entity）、常量或 Input 参数
└── 3. 子包内部自用的工具函数与局部数据结构
```

---

## 目录拓扑与依赖流向

```
internal/core/<domain>/         ← 领域根包（定义 Core、Storer 聚合接口、实体模型、port.go）
├── sub-a/                      ← 子包 A（单向依赖 domain 根包）
├── sub-b/                      ← 子包 B（单向依赖 domain 根包，可单向依赖 sub-a）
├── <provider>adapter/          ← 对外提供的适配器（依赖 domain 根包）
├── stores/<domain>db/          ← DB 存储实现（实现 domain.Storer 接口）
└── stores/<domain>cache/       ← Cache 存储实现（实现 domain.Storer 接口）
```

### 循环依赖解除示例

若 `sub-a` 需要调用 `sub-b` 的某些计算逻辑，而 `sub-b` 又需要通知 `sub-a` 状态变更：

1. `sub-a` 直接 import 并调用 `sub-b`。
2. 在 `sub-b` 中定义一个本地接口：
   ```go
   // sub-b/port.go
   type StateListener interface {
       OnStateChange(ctx context.Context, id string) error
   }
   ```
3. `sub-b` 通过构造函数或 Option 接收 `StateListener`，由外部（或 `sub-a`）在初始化时注入具体实现，从而打破直接包引用循环。
