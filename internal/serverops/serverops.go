// Package serverops provides core infrastructure for server operations including data persistence,
// state management, error handling, and security utilities and other primitives or wiring for libraries.
//
// Subpackages are prohibited from cross-importing. Shared utilities
// in other words: subpackages of serverops are NEVER allowed to use other subpackages of serverops.
package serverops
