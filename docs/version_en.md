# About GORM AutoMigrate Control for Faster Startup

## What is AutoMigrate?

In Go development, we often need to modify database table structures. The traditional approach involves manually writing SQL statements, which is both tedious and error-prone. GORM's AutoMigrate feature was created to solve this problem, automatically synchronizing code and database structures to make development more efficient.

## Why Control AutoMigrate?

Let's look at a real scenario: In the early stages of a project with just 7 tables, AutoMigrate execution takes only 1 second. As features are added and the number of tables grows to 30, startup time extends to 10 seconds. Even worse, every startup requires checking all tables, even when no structural changes have been made - this is clearly inefficient.

## Solution: Version Control

goddd introduces a version control mechanism, similar to software updates, where updates are only needed when a new version is released. Implementation details:

1. Database records the current version number
2. Program checks code version number at startup
3. AutoMigrate is only executed when code version number is greater than database version number

## How to Modify Version? Code Example

```go
// Set database version (both 0.0.2 and v0.0.2 are acceptable)
versionapi.DBVersion = "0.0.2"
// Add version description to record what was updated
versionapi.DBRemark = "Added user avatar field"
```

The `orm.SetEnabledAutoMigrate` variable can be used to globally control AutoMigrate's enabled state.

## What are the Benefits?

1. Faster program startup by avoiding unnecessary table checks
2. Reduced database pressure by eliminating redundant operations
3. Clear tracking of database structure change history

## About goddd

[github.com/ixugo/goddd](https://github.com/ixugo/goddd) is a Go language project template based on Domain-Driven Design (DDD), providing a complete project structure and best practices. It integrates commonly used components like GORM and Gin, making it particularly suitable for quickly starting small to medium-sized projects.

Want to learn more? Check out the goddd source code.