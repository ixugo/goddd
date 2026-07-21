---
name: goddd-api-doc
description: >
  Apifox 同步、上传接口文档到 Apifox、生成或更新 OpenAPI 3.1 YAML 接口文档。
  当用户提到 apifox、同步 apifox、更新到 apifox、上传到 apifox、.go.yaml 文件、
  接口文档、同步文档、生成文档时使用此技能。
  也在项目使用 web.WrapH 注册路由且 API 层发生变动时自动触发。
  docs/api/*.go.yaml 文件的任何操作都应触发此技能。
---

# 更新接口文档

根据 Go Gin 路由代码自动生成或增量更新 OpenAPI 3.1 文档；接口或结构发生变更时需同步更新文档并记录字段变更历史。

## 参考资料索引

设计接口或编写文档前，先查阅以下参考文件：

| 文件 | 用途 |
|------|------|
| `goddd/references/api-design-patterns.md` | API 设计规范（位于 goddd 技能目录）：资源命名、HTTP 方法、状态码、分页、版本控制、限流策略 |
| `references/openapi-example.yaml` | OpenAPI 3.1 文档示例模板，包含完整的 CRUD 接口、schema 定义和字段变更记录写法 |

## 适用范围

- 路由、HTTP 方法、路径变更
- 入参/出参结构体增删改（含嵌入字段变更）
- domain/model 字段与 JSON 映射变更（字段重命名、删除、类型变更）
- 字段发生上述变动时，需在文档中记录更新说明（见「字段变更记录」）

## 字段变更记录

当某个字段被**修改**、**删除**或**重命名**时，必须在接口文档中记录更新时间和更新内容。

- **记录位置**：写在 **paths 下具体接口**的描述里，即 `paths[path][method].description`（哪个 path、哪个 method，就写在对应该接口的 description 中）。不要写在 `info.description`。具体字段的 `description` 仅描述用途、中文名、取值范围等，不混入变更记录。
- **格式**：日期后**换行**，用 **Markdown 无序列表**（`-`）列出变更项。
- **YAML 示例**（在某个接口的 description 中）：
```yaml
paths:
  /messages:
    get:
      description: |
        查询当前登录用户的消息列表，包含内容详情与发送者信息
        更新说明：
        2026-03-03
        - ref_id 修改为 target_id
```
- **必须记录的情况**：字段重命名、字段删除、类型变更、必填/可选变更。

## 路由变更与弃用

当代码中某个接口的**路由路径或 HTTP 方法**发生变更时（如 `/tenants/:id` 改为 `/tenants/{id}`，或 `GET` 改为 `POST`），**不得删除文档中的旧接口**，必须按以下规则处理：

1. **旧接口标记弃用**：在旧路径/方法的定义中添加 `deprecated: true`，并在其 `description` 中追加弃用说明，指明替代的新接口。
2. **新接口正常补充**：按常规流程在文档中添加新路由/方法的接口定义。
3. **代码中不再存在的接口也需标记弃用**：若文档中的接口在代码里已无对应逻辑，同样标记 `deprecated: true` 并说明"代码中已移除"，**除非用户明确要求删除**。
4. **完成汇报**：每次更新文档后，必须向用户汇报本次操作涉及的弃用接口数量及列表（路径+方法）。

**YAML 示例**：

```yaml
# 假设路由从 GET /tenants/:id 变更为 GET /tenants/{id}/info
paths:
  /tenants/{id}:
    get:
      deprecated: true
      description: |
        获取多租户信息
        弃用说明：
        2026-06-12
        - 路由变更为 GET /tenants/{id}/info，请使用新接口
      # ...旧接口其余定义保留不变

  /tenants/{id}/info:
    get:
      description: |
        获取多租户信息（新路径）
      # ...新接口定义
```

**判断标准**：

- 路由路径变更（含路径参数格式变化）→ 旧接口标弃用 + 新增接口
- HTTP 方法变更（如 GET → POST）→ 旧方法标弃用 + 新增方法
- 接口从代码中彻底移除（无对应 handler）→ 标记弃用（`deprecated: true`），除非用户明确要求删除
- 字段/参数变更 → 走「字段变更记录」规则，不涉及弃用标记

## 文件约定

- 文档统一输出到项目根目录的 `docs/api/` 目录，**不与源文件同目录**
- 文件名：`{被操作的源文件名及后缀}.yaml`
- 例：`internal/web/api/media.go` → `docs/api/media.go.yaml`
- 若 `docs/api/` 目录不存在，执行前先创建：`mkdir -p docs/api`
- `docs/api/*.go.yaml` 已被 `.gitignore` 忽略，**检查文件是否存在时必须用 `ls` 命令**，不能用 Glob/Grep（Glob/Grep 会跳过 gitignore 的文件，导致误判为不存在）

## 提取规则

### 路由解析

```go
func RegisterTenant(g gin.IRouter, api TenantAPI, handler ...gin.HandlerFunc) {
    group := g.Group("/tenants", handler...)
    group.GET("/:id", web.WrapH(api.getTenant))
}
```

- `group.GET("/:id", ...)` → `GET /tenants/{id}`，路由参数为 `id`

### 参数提取

```go
func (a TenantAPI) getTenant(c *gin.Context, _ *struct{}) (*tenant.Tenant, error)
```

- `_ *struct{}` → 无 query/body 参数
- `*struct{ Name string \`form:"name"\` }` → query 参数
- `*struct{ Name string \`json:"name"\` }` → JSON body 参数
- `struct{ ID string \`uri:"id"\` }` → 路由路径参数（通过 `uri` tag 自动绑定）

### 映射与格式规则

1. `json:"-"` 标记的字段忽略，不出现在文档中 —— 这些字段不参与 JSON 序列化，调用方永远看不到
2. `orm.Time` 类型作为请求参数时映射为 `integer`（毫秒时间戳） —— 该类型的 MarshalJSON 输出为毫秒整数
3. Go 匿名嵌入字段展开为平级，不增加嵌套层 —— JSON 序列化时嵌入字段会被提升到外层
4. 路由函数顶部注释作为接口描述（如 `// getTenant 获取多租户信息` → 描述为"获取多租户信息"）
5. 文件已存在时，对比代码变更后增量更新或覆盖重写
6. 自定义 `MarshalJSON` 的类型：以 JSON 序列化后的实际输出类型为准（如 Go 底层 `int` 但序列化为 `string`，文档应写 `type: string`） —— 文档描述的是调用方看到的 JSON，不是 Go 内部类型
7. 接口文档面向调用者，描述中不得出现数据库存储类型、内部序列化机制、表结构设计等服务端信息 —— 避免泄露实现细节，仅描述字段用途、取值范围和使用注意事项

### 公共类型

`web.PagerFilter`:
```yaml
page:
  type: integer
  description: 页码
size:
  type: integer
  description: 每页数量
sort:
  type: string
  description: 排序字段
```

`web.DateFilter`:
```yaml
start_ms:
  type: integer
  description: 开始时间（毫秒时间戳）
end_ms:
  type: integer
  description: 结束时间（毫秒时间戳）
```

## 跨文件引用禁令

**禁止使用跨文件 `$ref`**（如 `$ref: "group.go.yaml#/components/schemas/Sms2wayGroup"`）。Apifox 通过 OpenAPI import 同步时，跨文件 `$ref` 无法被解析，会导致被引用的 schema 在 UI 上只显示 `object` 类型名，丢失所有字段描述。

**处理方式**：

1. **同一 Go 源文件的接口**：schema 定义在同一个 `.go.yaml` 文件内，使用本文件内 `$ref`（如 `$ref: "#/components/schemas/Foo"`）。
2. **不同 Go 源文件操作同一张表（共享 model）**：将这些接口合并到同一个 `.go.yaml` 文件中，通过不同的 `tags` 区分功能分组。
   - 例：`group.go` 和 `group.go` 中的规则接口操作同一张 `sms2way_group` 表，应合并为一个 `group.go.yaml`，分别挂 `admin/分组管理` 和 `admin/规则配置` 两个 tag。
3. **确需复用 schema 但不能合并文件**：将被引用的 schema 完整复制到引用方的 `components/schemas` 中（冗余优于跨文件引用），并在复制处加注释说明来源。

## 字段命名约定

- 文档中所有 JSON 属性键（properties key）**必须使用蛇形小写（snake_case）**，例如 `access_key`、`group_code`、`api_permissions`。
- 禁止在文档中出现小驼峰（`accessKey`）或大驼峰（`AccessKey`）形式，以与 Go 蛇形 JSON tag 保持一致。

## Tag 规则

- **全新生成**时，tag 格式为 `admin/<中文功能名>`，例如 `admin/应用管理`、`admin/分组管理`。
- **增量更新**已存在文档时，**禁止修改已有的 `tags` 字段**，除非用户明确指示更改。

## 工作流程

1. 读取目标 Go 源文件，提取路由注册和处理函数
2. 追踪参数结构体定义，展开嵌入字段
3. 确认 `docs/api/` 目录存在（不存在则创建）
4. 用 `ls docs/api/` 检查目标 `.go.yaml` 是否已存在
5. 已存在则对比更新（保留已有 tags 不变），不存在则全新生成
6. 将 YAML 文档写入 `docs/api/{源文件名}.yaml`
7. 满足条件时自动执行 Apifox 同步（见末尾章节）

---

## Apifox 同步

文档写入后，若满足以下条件，自动执行 `references/apifox.sh` 将变更同步到 Apifox：

- **条件 A**：环境变量 `APIFOX_TOKEN` 已设置
- **条件 B**：项目 `CLAUDE.md` 或 `AGENTS.md` 中定义了 `APIFOX_PROJECT_ID`

**执行命令**：

```bash
bash references/apifox.sh projectid=<PROJECT_ID> token=${APIFOX_TOKEN} file=./docs/api/<被改动的文件>
```

执行时 cwd 应为本技能所在目录（`.cursor/skills/goddd-api-doc/`）。不满足条件时跳过同步，不报错。
