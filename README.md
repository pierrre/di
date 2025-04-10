# DI

Simple Go depency injection container with generics.

[![Go Reference](https://pkg.go.dev/badge/github.com/pierrre/di.svg)](https://pkg.go.dev/github.com/pierrre/di)

## Features

- Set and get services to a container
- Lazy service instantiation
- Optional service name
- Close all initialized services
- Type safe (uses generics)
- Detect dependency cycle (error)
- [Service provider](https://pkg.go.dev/github.com/pierrre/di/diprovider#example-package) (helps to break circular dependencies)
- [Dependency graph](https://pkg.go.dev/github.com/pierrre/di#example-Dependency)

## Usage

[Example](https://pkg.go.dev/github.com/pierrre/di#example-package)
