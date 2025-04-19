# GoDDD: Enterprise REST API Development Template

## Introduction

[GoDDD](https://github.com/ixugo/goddd) is an enterprise-level template focused on REST API development, designed to provide Go developers with a complete CRUD solution. It adopts Domain-Driven Design (DDD) principles and a modular monolithic architecture, enabling developers to quickly start projects and focus on business development.

## Why Choose GoDDD?

As a Go developer, have you ever encountered these challenges:

- Project structure becomes increasingly difficult to maintain as the business grows
- New team members take a long time to understand the project
- Frequent code conflicts during team collaboration
- Need to write large amounts of similar CRUD code repeatedly
- Inconsistent error handling and messy logging
- Require extensive infrastructure configuration when starting a project

GoDDD is designed to solve these problems. It provides a clear project structure and a complete development toolchain, including:

- Modular project structure for better code organization
- Unified error handling and logging
- Automated code generation tool [godddx](https://github.com/ixugo/godddx)

## Technical Architecture

### Architecture Design Philosophy

In traditional MVC monolithic architecture, as business scale grows, projects become increasingly bloated, leading to reduced team development efficiency and difficulty for new members to quickly understand the system.

GoDDD adopts a modular monolithic architecture that preserves the simplicity of monolithic architecture while incorporating the modular advantages of microservices. By breaking down the business into independent domain modules (such as user domain, banking domain, product domain, etc.), each domain contains a complete set of:

- API (Interface Layer)
- Core (Business Core)
- Store (Data Storage)

This design allows different teams to independently develop and maintain their domain modules, effectively reducing code conflicts and development chaos. Compared to microservices, this modular approach makes the code more concise, easier to test, and maintain.

More importantly, when a domain module's scale exceeds expectations, teams can easily extract it as an independent microservice, achieving smooth architecture evolution.

### Technology Stack

- **Web Framework**: Gin
- **ORM**: GORM
- **Architecture Design**: Domain-Driven Design (DDD)
- **Code Generation**: Support for automated code generation
- **Message Processing**: Event bus/transaction messages using NSQite

## Project Structure

```
.
├── cmd               # Executable programs
├── configs           # Configuration files
├── makefile          # Provides build/image/toolchain commands
├── internal          # Private business logic
│   ├── conf          # Configuration models
│   ├── core          # Business domains
|   ├── domain        # Public domains, providing modular components for rapid development
|	├── data 		  # Database initialization logic
│   └── web           # Public Web layer
└── pkg               # Dependency libraries
```

## Application Cases

- GB/T28181 Video Platform (github.com/gowvp/gb28181)

## Conclusion

For junior Go developers, GoDDD provides a clear project structure and complete development toolchain, enabling you to quickly get started with enterprise-level project development. It solves common problems such as messy project structure, difficult team collaboration, and code duplication, allowing developers to focus on business logic implementation.

If you're looking for a framework to help you quickly build enterprise-level REST APIs, GoDDD is definitely worth trying. Its modular design and complete toolchain can significantly improve your development efficiency and code quality.