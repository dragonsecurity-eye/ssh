package ssh

import (
	"testing"
)

func TestContextAccessorsBeforeMetadata(t *testing.T) {
	// Context accessors should return zero values before metadata is applied
	ctx, cancel := newContext(nil)
	defer cancel()

	if got := ctx.User(); got != "" {
		t.Fatalf("User() = %q; want empty", got)
	}
	if got := ctx.SessionID(); got != "" {
		t.Fatalf("SessionID() = %q; want empty", got)
	}
	if got := ctx.ClientVersion(); got != "" {
		t.Fatalf("ClientVersion() = %q; want empty", got)
	}
	if got := ctx.ServerVersion(); got != "" {
		t.Fatalf("ServerVersion() = %q; want empty", got)
	}
	if got := ctx.RemoteAddr(); got != nil {
		t.Fatalf("RemoteAddr() = %v; want nil", got)
	}
	if got := ctx.LocalAddr(); got != nil {
		t.Fatalf("LocalAddr() = %v; want nil", got)
	}
}

func TestContextPermissions(t *testing.T) {
	ctx, cancel := newContext(nil)
	defer cancel()

	perms := ctx.Permissions()
	if perms == nil {
		t.Fatal("Permissions() should not be nil for new context")
	}
}

func TestContextSetAndGetValue(t *testing.T) {
	ctx, cancel := newContext(nil)
	defer cancel()

	ctx.SetValue("custom-key", "custom-value")
	got := ctx.Value("custom-key")
	if got != "custom-value" {
		t.Fatalf("Value() = %v; want 'custom-value'", got)
	}
}

func TestContextInternalMutex(t *testing.T) {
	ctx, cancel := newContext(nil)
	defer cancel()

	// The internal mutex should not deadlock on concurrent SetValue
	ctx.SetValue("a", 1)
	ctx.SetValue("b", 2)
	if ctx.Value("a") != 1 || ctx.Value("b") != 2 {
		t.Fatal("expected values to be set")
	}
}
