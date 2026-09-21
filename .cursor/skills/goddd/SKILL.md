---
name: goddd
description: >
  在已使用 GoDDD 的 Go 项目中实现或维护领域、CRUD、Store、API 与依赖装配，
  或在用户明确要求采用 GoDDD 时提供架构迁移指导。
  通用 DDD、缓存和 Go 包依赖问题不单独触发本技能。
---

# GoDDD 开发指南

先确认项目采用的 GoDDD 版本、现有实现与本次任务范围，再按下表读取直接相关的专题，不默认加载全部参考文档。API 签名、生成器行为和框架限制以目标项目源码为准；文档示例用于说明设计，不替代版本核查。

## 使用边界

- 新建功能沿用项目分层；维护已有功能时保留公开协议、查询范围、事务与错误语义。
- 仅在用户要求迁移时应用迁移清单；发现旧式目录、方法名称或生命周期字段，不自动重构。
- 优先复用语义等价的现有方法。是否等价取决于唯一约束、租户与权限范围、过滤条件和返回契约，不取决于名称相似。
- 示例中的数据库、缓存及异步队列选型不是安装依赖或迁移基础设施的授权。

## 分层边界

依赖方向为 `API → Core ← Store/Adapter`。API 处理协议与已验证的请求身份；Core 处理业务规则，通过 Storer、Port 使用外部能力。Store/Adapter 实现持久化及外部集成。

`internal/core/<domain>/` 保存领域接口、模型和业务方法，`stores/<domain>db/`、`stores/<domain>cache/` 保存存储实现，`internal/web/api/` 保存 HTTP 适配与装配。Core 可使用框架的 `orm.Tx` 抽象及模型映射标签，不直接操作 `*gorm.DB` 或依赖具体 Store 实现。具体目录与接口约定见领域专题。

## 专题索引

| 文档路径 | 读取时机 | 核心内容 |
|---------|---------|---------|
| `references/code-generation.md` | 新增 CRUD、定义数据库表模型、Wire 注册 | `tables/` 目录规范、主键与时间戳约束、`goddd gen` 命令、Wire 注入、`wire_gen.go` 严禁手改、路由注册 |
| `references/domain-layer-architecture.md` | 实现/修改领域 Core、Store 接口与事务 | Storer 聚合接口、EntityStorer 规范、事务机制（`Begin/WithTx`）、访问器零分配原理、原子更新、幂等删除、Input 参数定义 |
| `references/api-design-patterns.md` | 设计新接口、审查 API 规范、错误映射 | 资源命名、标准方法（List/Get/Create/Update/Delete）、自定义方法、错误体系、分页过滤、校验、限流 |
| `references/web-toolkit.md` | 使用 `pkg/web` 中的工具函数与中间件 | WrapH 绑定规则、PagerFilter/DateFilter、JWT 鉴权、日志/限流/SSE 中间件、Validator |
| `references/adapter-pattern.md` | 新增跨领域依赖、跨域事务协调 | 窄接口归属、适配器使用边界、错误契约、事务协调 |
| `references/cache-layer.md` | 修改或扩展 `stores/<domain>cache/` | 内存 vs Redis 缓存选型、回填与写入竞态边界、singleflight 防击穿、WarmUp 预热、事务副本语义 |
| `references/package-dependency.md` | 领域内拆分子包、处理子包间依赖 | 子包单向依赖根包原则、同级子包直接引用、Narrow Interface 打破双向循环依赖 |
| `references/event-notification.md` | 跨领域异步通知、解耦副作用 | `pkg/event` 泛型总线、观察者注册、Wire 注入、River 持久化异步队列集成 |
| `references/sort.md` | 实现列表拖拽重排序 | 同事务锁定与读取、排序值重分配、唯一约束与并发边界 |
| `references/with-context.md` | Core 或 Adapter 需要 HTTP 上下文信息 | `web.WithContext` 包装 → Core 透传标准 `ctx` → Adapter 类型断言解包 |
| `references/lifecycle-split.md` | 已确认需要拆分 Core 生命周期或解决 Wire 环路 | 依赖图、构造与启动分离、Handler 边界、关闭与错误处理 |
| `references/refactor-migration.md` | 重构/迁移旧代码到 goddd 架构 | SQL 条件、默认排序、错误控制流、空列表序列化、缓存失效等价性检查清单 |


## 文档联动

接口契约变更时更新项目已有的 API 文档。项目采用 `docs/api/*.go.yaml` 且环境提供 `goddd-api-doc` 时，按该技能处理对应文档；未采用该体系时沿用项目既有流程。更新本地文档不代表获准向 Apifox 等外部服务上传。
