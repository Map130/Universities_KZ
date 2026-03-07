//go:build !noauth

package auth

import (
	"github.com/gofiber/fiber/v2"
)

// IsNoAuth reports whether the application was built without authentication.
// In the default (production) build this always returns false.
func IsNoAuth() bool { return false }

// NoAuthMiddleware is a stub that exists only to satisfy the compiler.
// It is never called in the production build because IsNoAuth() returns false.
// If called by mistake, it panics to prevent silent security bypass.
func NoAuthMiddleware() fiber.Handler {
	panic("auth: NoAuthMiddleware called in production build — this should never happen")
}
