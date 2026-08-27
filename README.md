<p align="center">
    <img src="./logo.png#gh-light-mode-only" alt="GoDDD Logo" width="550"/>
    <img src="./logo_dark.png#gh-dark-mode-only" alt="GoDDD Logo" width="550"/>
</p>

<p align="center">
    <a href="https://github.com/ixugo/goddd/releases"><img src="https://img.shields.io/github/v/release/ixugo/goddd?include_prereleases" alt="Version"/></a>
    <a href="https://github.com/ixugo/goddd/blob/master/LICENSE.txt"><img src="https://img.shields.io/dub/l/vibe-d.svg" alt="License"/></a>
	<a href="https://gin-gonic.com"><img width=30px src="https://avatars.githubusercontent.com/u/7894478?s=48&v=4" alt="GIN"/></a>
    <a href="https://gorm.io"><img width=70px src="https://gorm.io/gorm.svg" alt="GORM"/></a>

</p>

[English](./README.md) | [简体中文](./README_zh.md)

# AI-Driven Lightweight Enterprise REST API Scaffold

This is a complete CRUD solution focused on REST API.

GoDDD is an **AI-driven lightweight enterprise REST API scaffold**. By describing business requirements in natural language, [godddx](https://github.com/ixugo/godddx) generates domain code for you, so developers can focus on business logic instead of boilerplate.

The goal of GoDDD is to:

+ Provide a clean architecture suitable for projects of any size.
+ Provide a modular structure for quickly starting a project, focusing on business development.
+ Simplify projects, making development more efficient and enjoyable.
+ Keep the learning curve low, so developers can get started quickly without being DDD experts.

If you think the above description fits your needs, then let's get started quickly.

Supports code generation: built-in `goddd gen` command or standalone tool [godddx](https://github.com/ixugo/godddx).

Supports [event bus/transaction messages](https://github.com/ixugo/nsqite).

## Design Description

Traditional MVC monolithic architectures become increasingly difficult to develop as the business grows, and new team members struggle to understand the bloated monolith.

A modular monolithic architecture retains many of the advantages and disadvantages of a monolith, while gaining most of the benefits and only a few of the drawbacks of microservices.

The complete business is split into multiple domain modules, such as User Domain, Bank Domain, and Product Domain. Each domain has its own complete set of:

+ API (interfaces)
+ Core (business logic)
+ Store (cache / persistence)

Different developers or teams can work on these domain modules independently, reducing confusion and conflicts caused by adding new features. Compared to microservices, modules organized this way are smaller, cleaner, and easier to test.

When the program outgrows the domain-module scale, the team can easily extract a domain module into a microservice when needed.

## Quick Start

### Option 1: Using goddd CLI (Recommended)

```bash
# Install CLI
go install github.com/ixugo/goddd/cmd/goddd@latest

# Create a new project
goddd init myapp -g github.com/yourname/myapp

# Run the project
cd myapp && go run main.go
```

Generate DDD layered code:

```bash
# Generate Core/Store/Cache/API code from struct definitions
goddd gen -f tables/user/user.go

# Multiple files
goddd gen -f tables/user/user.go,tables/task/task.go
```

For more commands, see [cmd/goddd/README.md](cmd/goddd/README.md).

### Option 2: Manual Clone

```bash
git clone --depth 1 --branch template-empty https://github.com/ixugo/goddd.git myapp
cd myapp
rm -rf .git && git init
```

1. Update the module path in `go.mod` to your actual module path
2. `make init` (optional — installs development tools)
3. `go build -o server ./main.go && ./server`
4. Open a new terminal and access `curl http://localhost:8080/health`




## Team Standards

| Standard | Description | Document |
|----------|-------------|----------|
| API Design | Resource naming, HTTP methods, status codes, pagination, versioning | [api-design-patterns.md](.cursor/skills/goddd/references/api-design-patterns.md) |
| Git Workflow | Commit conventions, branching strategy, versioning, changelog | [goddd-git-workflow](.cursor/skills/goddd-git-workflow/SKILL.md) |

## API Documentation

The project uses an AI-driven code-first approach to generate OpenAPI 3.1 documentation without manual annotations. When API layer code changes, interface definitions are automatically extracted from route registration (`web.WrapH`), handler signatures, and struct tags, generating YAML documents to the `docs/api/` directory.

See [goddd-api-doc skill](.cursor/skills/goddd-api-doc/SKILL.md) for details.

**Apifox Auto-Sync**

Generated documents can be automatically pushed to Apifox. Configuration required:

1. Set environment variable `APIFOX_TOKEN` (Apifox personal access token)
2. Set `APIFOX_PROJECT_ID=<your-project-id>` in `CLAUDE.md` or `AGENTS.md` at the project root

Once configured, the sync script runs automatically on document changes. Skipped silently when unconfigured.

## Naming Conventions

| Category | Convention | Example |
|----------|-----------|---------|
| Domain directory | Lowercase, no separators | `version`, `tenant` |
| Domain files | `<domain>.<purpose>.go` | `version.model.go`, `version.param.go` |
| Store directory | `stores/<domain>db` | `stores/versiondb` |
| Store files | `db.go` (entry/migration) + `<domain>.go` (CRUD) | `db.go`, `version.go` |
| Domain model | PascalCase | `Version`, `Tenant` |
| Input struct | `<Action><Domain>Input` | `CreateVersionInput` |
| Output struct | `<Action><Domain>Output` | `FindUserOutput` |
| Core object | Fixed `Core` | `type Core struct` |
| Core constructor | `NewCore(store Storer, opts ...Option)` | — |
| Store interface | Fixed `Storer` | `type Storer interface` |
| API struct | `<Domain>API` | `VersionAPI` |
| Route registration | `Register<Domain>(g gin.IRouter, api <Domain>API, handler ...gin.HandlerFunc)` | `RegisterVersion(...)` |
| JSON tag | snake_case | `json:"created_at"` |
| Route path | kebab-case + plural nouns | `/api/v1/users` |
| Database table | snake_case + plural | `versions` |

## Security

**SQL Injection Prevention**

All database operations must use GORM parameterized queries. String concatenation is prohibited:

```go
// Correct: parameterized query
db.Where("name = ?", input.Name).First(&user)

// Wrong: string concatenation
db.Where("name = '" + input.Name + "'").First(&user)
```

**Log Sanitization**

Sensitive user information in log output must be masked:

| Field Type | Masking Rule | Example |
|------------|-------------|---------|
| Phone | Replace middle 4 digits with `****` | `138****1234` |
| Email | Keep only first and last before `@` | `a***z@example.com` |
| Password/Token | Never write to logs | — |
| ID Number | Keep first 3 and last 4 | `110***1234` |

**Sensitive Data Encryption**

- Passwords must use `bcrypt`, no plaintext or MD5
- Token/key configurations injected via environment variables, no hardcoding
- `.env`, `credentials.json` etc. must be in `.gitignore`

## Unit Testing

Core business logic (exported Core functions) must have unit tests covering key paths:

- Store interfaces naturally support mocking — replace with mock implementations in tests
- Test files go in the same directory as the source, named `<file>_test.go`
- Fix bugs by writing a reproduction test first, then fixing

Not required for: pure DTO definitions, simple getters/setters, direct CRUD pass-through in Store.

## Cross-Domain Aggregation

When business logic involves data from multiple domains, choose the appropriate approach based on performance requirements and coupling constraints:

| Pattern | Coupling | Use Case |
|---------|----------|----------|
| SQL Pattern | High | Query aggregation — Store layer writes JOIN queries across domain tables |
| Command Pattern (WithTx) | Medium | Write aggregation — share a single transaction via `orm.Tx` + `WithTx` |
| API Layer Aggregation | Medium | API layer coordinates multiple Cores, each runs independently, results assembled in API layer |
| Adapter Pattern | Low | Domains decoupled via Option-injected interfaces, each manages its own transactions |
| Event Notification (Observer) | Low | One-to-many sync/async broadcast via `pkg/event.Bus[T]` generics |

**SQL Pattern**

Write JOIN queries directly in the Store layer for read-only query aggregation:

```go
func (d OrderDB) FindOrdersWithUser(ctx context.Context, userID string) ([]OrderWithUser, error) {
    var result []OrderWithUser
    return result, d.db.WithContext(ctx).
        Table("orders").
        Select("orders.*, users.name as user_name").
        Joins("LEFT JOIN users ON users.id = orders.user_id").
        Where("orders.user_id = ?", userID).
        Find(&result).Error
}
```

**Command Pattern (WithTx)**

Start a transaction via `Storer.Begin()`, create transactional Store copies with `WithTx`:

```go
func (c Core) CreateOrderAndDeduct(ctx context.Context, in CreateOrderInput) error {
    tx, err := c.store.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    txOrder, _ := c.store.Order().WithTx(tx)
    txStock, _ := c.store.Stock().WithTx(tx)

    if err := txOrder.Create(ctx, in.Order); err != nil {
        return err
    }
    if err := txStock.Deduct(ctx, in.ProductID, in.Quantity); err != nil {
        return err
    }
    return tx.Commit()
}
```

**API Layer Aggregation**

API layer calls multiple Cores and assembles results. For lists, attach related data as a map to avoid N+1 queries:

```go
func (a OrderAPI) listOrders(c *gin.Context, in *ListOrderInput) (*ListOrderOutput, error) {
    ctx := c.Request.Context()
    orders, err := a.orderCore.ListOrders(ctx, in.Pager)
    if err != nil {
        return nil, err
    }
    userIDs := uniqueUserIDs(orders) // extract deduplicated user IDs
    users, _ := a.userCore.GetUserMap(ctx, userIDs)
    return &ListOrderOutput{
        Items: orders,
        Users: users, // map[string]UserBrief{"user_xxx": {Name: "Zhang San"}}
    }, nil
}
```

**Adapter Pattern**

Domains fully decoupled via Option-injected adapter interfaces.

> See [adapter-pattern.md](.cursor/skills/goddd/references/adapter-pattern.md) for details.

## References

[Google API Design Guide](https://google-cloud.gitbook.io/api-design-guide)

Best Practices for This Project: https://github.com/gowvp/gb28181


## Directory Structure

```bash
.
├── main.go                 # Main entry point
├── cmd                     # Executable entry points
├── configs                 # Configuration files
├── docs                    # Design / user documentation
├── domain                  # Common domains and basic business
│   ├── token               # JWT tokens and permissions
│   ├── uniqueid            # Global unique ID generator
│   └── version             # DB schema version control to avoid gorm migration on every start
│       ├── stores/versiondb
│       └── versionapi
├── internal                # Private business code
│   ├── app                 # Wire dependency-injection assembly
│   ├── conf                # Configuration models and defaults
│   ├── core                # Business domain (example / reserved)
│   ├── data                # Database initialization
│   └── web
│       └── api             # RESTful API route registration
├── pkg                     # Project libraries
│   ├── cmd                 # CLI helpers
│   ├── conc                # Concurrency utilities
│   ├── hook                # Common function hooks
│   ├── logger              # Logging wrapper
│   ├── orm                 # ORM wrapper
│   ├── queue               # Queue implementations
│   ├── reason              # Error reason definitions
│   ├── server              # HTTP server wrapper
│   ├── system              # System utilities
│   └── web                 # Web middleware, responses, validation, etc.
└── tables                  # Auto-generated business table models (example)
```

## Project Description

1. Components strongly relied upon by the program will trigger a panic on error, so that issues are resolved as quickly as possible.

2. The core directory represents the business domain, containing domain models and domain business functions.

3. The store is the database operation module, dependent on models with dependency inversion towards the core, avoiding the need to define models at each layer.

4. Input/output parameters in the API layer may directly depend on models defined in the core layer, with input and output models distinguished by appending `Input/Output` to the model names.


## Request Parameter Wrapping

This project uses GIN as the web framework, and the route functions need to implement `gin.HandlerFunc`. The first issue encountered when implementing API functions is binding parameters. Almost every function involves deserialization, and the function heads are cluttered with `ctx.ShouldBindJSON` and similar code.

To follow the DRY (Don't Repeat Yourself) design principle, we reduce repetitive code to improve maintainability and reusability. The project wraps `web.WrapH`, which returns a `gin.HandlerFunc`. The parameters for `web.WrapH` are similar to gRPC, with a signature like `func(ctx *gin.Context, in *struct{}) (*Output, error)`.

`WrapH` internally recognizes POST/PUT/DELETE/PATCH requests and binds the Request Body, while GET requests bind Request URL parameters.

The second parameter of the input must be a pointer, and `*struct{}` is used when no parameters need to be bound. When defining the structure, especially note that the struct tags should be `json` or `form`. More details are available in the GIN framework's parameter binding documentation.

+ `json`: Can bind request body parameters.
+ `form`: Can bind query parameters.

The first parameter of the return value is the actual response body content, and it is recommended to avoid using `any`. The type can be either a value or a pointer, providing more flexibility.

When parameters exist in multiple places, such as route parameters, query parameters, and request body parameters, you can implement a new `web.WrapH2` or directly implement `gin.HandlerFunc`.

Here are two code examples:


```go
func findUser(ctx *gin.Context) {
	var in findUserInput
	if err := ctx.ShouldBindQuery(&in);err!=nil {
		ctx.JSON(...)
		return
	}
	out,err := serviceFunc(in)
	// ....
}
```

```go
func findUsers(ctx *gin.Context, in *Input) (*Output, error) {
	return serviceFunc(in)
}
```

## Response Parameter Wrapping

Clearly defining the response type can make the code easier to understand. The goal is to improve code readability and maintainability by paying attention to more details.

The web.WrapH wrapper defaults to returning a response with the application/json content type.

During development, new colleagues may forget the return statement when implementing gin.HandlerFunc. Using web.WrapH ensures that the return statement is not omitted.

Here are two code examples:

```go
func findUsers(ctx *gin.Context) {
	// Maybe out is obtained from the business layer
	// At this point, you need to find the response body inside the function
	out, err := serviceFunc()
	if err != nil {
		ctx.JSON(...)
		return
	}
	ctx.JSON(out)
}
```

```go
func findUsers(ctx *gin.Context, in *Input) (*Output, error) {
	return serviceFunc(in)
}
```

## Error Handling

From the above code, we can see that errors are directly returned. But doesn't this expose the underlying error information to the user? And what about the HTTP status code for errors?

In fact, `web.Warn` does some additional work. For example, when there is an error during binding, it can pinpoint the specific error cause: Is the type wrong? Which property is incorrect? For example, when responding, we can extract information from the `err` and return the corresponding HTTP status code. Let's take a closer look at error handling.

`pkg/web` is an HTTP-related handling package, which includes middleware, response handling, error handling, authentication, logging, rate limiting, metrics, performance analysis, input validation, and more.

We define a custom `Error` type, where `reason` represents the error cause. Some third-party APIs also use a `Code`.

When designing the project, we considered that status codes might be hard to interpret, for example, error `10020`—what does that error mean? Therefore, we defined `reason`, which should describe the error cause in a concise, camel-case English format. If you just want to use the status code, then use the HTTP StatusCode.

`msg` should be an error description in the developer's native language, while `reason` is used internally by the program, and `msg` is for user-friendly messaging. `details` is an extension of the error, providing additional information for developers. It can describe solutions to the error, provide documentation, give more detailed error information, or even expose lower-level errors.

In front-end and back-end separated projects, when the front-end encounters an error, they often need to ask the back-end what happened. Through `details`, the front-end can reduce the number of inquiries.

In the `web.WrapH` wrapper, errors are actually handled by calling `web.Fail(err)`. This method determines which HTTP status code should be returned based on the `reason`. Developers can implement more HTTP status code extensions in the `pkg/web/error.go` file through the `HTTPCode()` function. By default, three status codes are provided: 200, 400, and 401.

`details` should only be visible in development mode. You can set the release mode using `web.SetRelease()`, in which case `details` will not be included in the HTTP response body.


Functions exported from the core layer or errors returned from the API layer should return errors of type `reason.Error`.

In the wrapped `web.WrapH`, errors are correctly logged and returned to the front-end.

```go
func findUser(in *Input) (*Output, error) {
	// Database operation error
	if err != nil {
		return nil, reason.ErrDB.WithMsg() // The response type is a DB layer error, and the Msg function can modify the user-friendly message
	}
	// Business logic error
	if err != nil {
		return nil, reason.ErrServer.Withf("err[%s] ....", err) // Withf can write details to provide more hints to the developer
	}
}
```


## Makefile

**How to install make?** `claude -p "install make tool"` or search online. On Windows, use Git Bash to ensure consistent behavior with Linux commands — do not use the default cmd/powershell terminal.

Use `make` or `make help` to get more help.

When writing a Makefile, add comments above each command in the format `## <command>: <description>` for readability, with available parameters provided in the Makefile. The goal is to make `make help` output more informative.

Some default operations are provided in the Makefile to assist with rapid development.

`make confirm` confirms the next step.

`make title content=Title` highlights a title in the output.

`make info` fetches build version information.

**Versioning Rules in the Makefile**

1. Git tags are used for versioning, in the format v1.0.0.

2. If the current commit lacks a tag, the closest tag is found, and the number of commits from that tag is calculated. For example, if the latest tag is v1.0.1, and there have been 10 commits since, the version number becomes v1.0.11 (v1.0.1 + 10 commits).

3. If there are no tags, the default version is v0.0.0, with the minor version incremented based on the number of commits.


## How to use the library?

### hook.UseCache Temporary cache

old

```go
cache := make(map[string]string)
for i := range 10 {
	v, ok := cache[i]
	if ok {
		// Business processing
		continue
	}
	v,err := fn()
	if err == nil {
		cache[v.ID] = v
	}
	// Business processing
}
```

new

```go
	cacheFn :=  hook.UseCache(fn)
	for i := range 10 {
		v,_,err :=  cacheFn(i)
		if err == nil {
			// Business processing
		}
	}
```

### hook.UseTiming Log the cost of function computation

old

```go
	now := time.Now()
	// Business logic is intermingled with time calculation
	if sub :=time.Since(now); sub > time.Second {
		slog.Error("func name", "cost", cost)
	}else {
		slog.Debug("func name", "cost", cost)
	}
```

new

```go
	cost := hook.UseTiming(time.Second)
	defer cost()

	// Business processing
```

### hook.UseTimer Timer with flexible intervals

old

```go
func scheduleTask() {
	for {
		// Business processing
		processTask()

		// Complex time calculation logic mixed with business logic
		now := time.Now()
		nextRun := time.Date(now.Year(), now.Month(), now.Day()+1, 2, 0, 0, 0, now.Location()) // Run at 2 AM tomorrow
		if nextRun.Before(now) {
			nextRun = nextRun.Add(24 * time.Hour)
		}
		time.Sleep(nextRun.Sub(now))
	}
}
```

new

```go
func scheduleTask(ctx context.Context) {
	hook.UseTimer(ctx, processTask, func() time.Duration {
		return hook.NextTimeTomorrow(2, 0, 0) // Run at 2 AM every day
	})
}

// Or for immediate execution with different intervals
func scheduleTaskWithFirstRun(ctx context.Context) {
	nextTime := hook.NextTimeWithFirst(
		time.Second,        // Run immediately (after 1 second)
		func() time.Duration {
			return 10 * time.Minute // Then run every 10 minutes
		},
	)
	hook.UseTimer(ctx, processTask, nextTime)
}
```

**Check out the source code in pkg/hook for more hooks.**

### web.WithContext Pass HTTP metadata to the Core layer without coupling

In a hexagonal architecture, the Core layer should not depend on HTTP frameworks (e.g. `gin.Context`). However, some scenarios require HTTP request metadata to construct dynamic data (such as full image URLs or the current request's base address).

Common approaches each have drawbacks:

| Approach | Problem |
|----------|---------|
| Post-process each field in the API layer | Repetitive code scattered across handlers |
| Pass `*http.Request` in function signatures | Invasive, requires changing the entire call chain |
| Pass `baseURL string` as a parameter | Every new requirement adds another parameter |
| Pass `gin.Context` directly | Core layer becomes coupled to the HTTP framework |

`web.WithContext` solves this by extending `context.Context` with an interface that carries HTTP request information:

```go
type Context interface {
    context.Context
    Request() *http.Request
    GetBaseURL() string
    GetScheme() string
    GetHost() string
}
```

**Usage in the API layer** — just replace `c.Request.Context()` with `web.WithContext`:

```go
ctx := web.WithContext(c.Request)
out, err := core.DoSomething(ctx, input)
```

**Usage in the Adapter layer** — type-assert to access HTTP metadata on demand:

```go
if wc, ok := ctx.(web.Context); ok {
    baseURL := wc.GetBaseURL()
    // Use baseURL to build the full URL
}
```

The Core layer passes `ctx` through transparently, with zero awareness of HTTP.

**Design highlights:**

+ **Zero breaking changes** — `web.Context` implements `context.Context`, so existing `func(ctx context.Context, ...)` signatures work as-is.
+ **Graceful degradation** — When the type assertion fails (e.g. in cron jobs, unit tests, CLI tools), the code falls back to default values.
+ **Progressive adoption** — Only two places need modification: the call site (API layer) and the usage site (Adapter layer). Everything in between is untouched.
+ **Extensible** — `web.Context` is an interface. You can define sub-interfaces to carry additional metadata such as tenant ID.

**Applicable scenarios:** dynamic URL construction, request-level metadata passing (IP, User-Agent, etc.), cross-domain data assembly requiring HTTP context.

## Quick Start

Example business logic:

Assume we want to implement version management. The CRUD steps are as follows:

Under "internal" - "core," create the "version" directory, then create `model.go` and define the domain model representing the database table structure.

Create `core.go` and add the following content:

```go
package version

import (
	"fmt"
	"strings"
)

// Storer Interface for dependency inversion in data persistence.
type Storer interface {
	First(*Version) error
	Add(*Version) error
}

// Core Business object
type Core struct {
	Storer    Storer
}

// NewCore Creates a business object.
func NewCore(store Storer) *Core {
	return &Core{
		Storer: store,
	}
}

// IsAutoMigrate Checks if table migration is required
// Compares the hard-coded database table version with the stored version.
func (c *Core) IsAutoMigrate(currentVer, remark string) bool {
	var ver Version
	if err := c.Storer.First(&ver); err != nil {
		isMigrate := true
		c.IsMigrate = &isMigrate
		return isMigrate
	}
	isMigrate := compareVersionFunc(currentVer, ver.Version, func(a, b string) bool {
		return a > b
	})
	c.IsMigrate = &isMigrate
	return isMigrate
}

func compareVersionFunc(a, b string, f func(a, b string) bool) bool {
	s1 := versionToStr(a)
	s2 := versionToStr(b)
	if len(s1) != len(s2) {
		return true
	}
	return f(s1, s2)
}

func versionToStr(str string) string {
	var result strings.Builder
	arr := strings.Split(str, ".")
	for _, item := range arr {
		if idx := strings.Index(item, "-"); idx != -1 {
			item = item[0:idx]
		}
		result.WriteString(fmt.Sprintf("%03s", item))
	}
	return result.String()
}
```

Under "stores/versiondb," create the `db.go` file with the following content:

```go
type DB struct {
	db *gorm.DB
}

func NewDB(db *gorm.DB) DB {
	return DB{db: db}
}

// AutoMigrate Table migration.
func (d DB) AutoMigrate(ok bool) DB {
	if !ok {
		return d
	}
	if err := d.db.AutoMigrate(
		new(version.Version),
	); err != nil {
		panic(err)
	}
	return d
}

func (d DB) First(v *version.Version) error {
	return d.db.Order("id DESC").First(v).Error
}

func (d DB) Add(v *version.Version) error {
	return d.db.Create(v).Error
}
```

In the API layer, inject dependencies by adding a function in `web/api/provider.go` to inject the business object into Usecase:

```go
var ProviderSet = wire.NewSet(
	wire.Struct(new(Usecase), "*"),
	NewHTTPHandler,
	NewVersion,
)

func NewVersion(db *gorm.DB) *version.Core {
	vdb := versiondb.NewDB(db)
	core := version.NewCore(vdb)
	isOK := core.IsAutoMigrate(dbVersion, dbRemark)
	vdb.AutoMigrate(isOK)
	if isOK {
		slog.Info("Updating database schema")
		if err := core.RecordVersion(dbVersion, dbRemark); err != nil {
			slog.Error("RecordVersion", "err", err)
		}
	}
	return core
}
```

Create a new `version.go` file in the API layer with the following content:

```go
// VersionAPI Namespace for version business functions.
type VersionAPI struct {
	ver *version.Core
}

func NewVersionAPI(ver *version.Core) VersionAPI {
	return VersionAPI{ver: ver}
}
// registerVersion Registers business interface with the router.
func registerVersion(r gin.IRouter, verAPI VersionAPI, handler ...gin.HandlerFunc) {
	ver := r.Group("/version", handler...)
	ver.GET("", web.WrapH(verAPI.getVersion))
}

func (v VersionAPI) getVersion(_ *gin.Context, _ *struct{}) (any, error) {
	return gin.H{"msg": "test"}, nil
}
```

## FAQ

> Why not define models in each layer separately?

This is a trade-off between development efficiency and decoupling, balancing code readability and efficiency.

> Where should API layer parameter models and table mapping models be defined?

Understanding the dependency relationships between layers is crucial. The API directly depends on the core, while the DB layer is inverted to depend on the core. Thus, domain models are defined in the core, and input/output parameter models can also be defined in the core. If they are unused in the core, defining them in the API layer is fine too.

> Why does the API layer directly depend on the core layer rather than an interface?

Interfaces aim to decouple, but in practice, it is more common to replace the API layer than the core layer.

The API only retrieves parameters and returns response parameters, doing the minimum necessary to facilitate the transition from HTTP to GRPC.

Design for the future, but program for the present. Increasing development efficiency now allows for a better approach in the future when needed.

> Why is the DB layer inverted to depend on the core?

Data persistence is not independent; it serves the business. That is, persistence serves the

 business and depends on it.

Through dependency inversion, other databases, such as Redis cache, can be inserted between business operations.

> Why suffix input/output models with `Input/Output`?

Convention is preferable to configuration. Some projects use `Request/Response` as suffixes to standardize parameter names.

Of course, output parameters can also serve as input, and you can define an alias or use them directly.

Frequently, we want clarity on what we're doing and why. This FAQ aims to offer some insight.

> How to write business plugins for GoDDD?

```go
// RegisterVersion Some general business functions are depended upon by other business functions, such as table version control, dictionary, verification code, scheduled tasks, user management, etc.
// Conventionally, write functions in the format Register<Core>, injecting three parameters: gin router, namespace, and middleware.
// Refer to project code for specifics.
func RegisterVersion(r gin.IRouter, verAPI VersionAPI, handler ...gin.HandlerFunc) {
	ver := r.Group("/version", handler...)
	ver.GET("", web.WrapH(verAPI.getVersion))
}
```

## Table Migration

Executing table migration on every program start is too slow.

Therefore, migration control is implemented through the version table, so migration only occurs when the database table version is outdated. Modify the `dbVersion` in api/db.go to control the version number.


## Custom Configuration Directory

The default configuration directory is `configs`, located in the same directory as the executable. You can also specify other configuration directories.

`./bin -conf ./configs`

## Main Project Dependencies

+ gin
+ gorm
+ slog / zap
+ wire