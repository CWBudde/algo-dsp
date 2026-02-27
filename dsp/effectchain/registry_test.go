package effectchain

import (
	"errors"
	"testing"
)

func dummyFactory(_ Context) (Runtime, error) {
	return &stubRuntime{}, nil
}

func TestRegistryRegister(t *testing.T) {
	t.Parallel()

	t.Run("registers and looks up factory", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()

		err := r.Register("chorus", dummyFactory)
		if err != nil {
			t.Fatalf("Register returned unexpected error: %v", err)
		}

		f := r.Lookup("chorus")
		if f == nil {
			t.Fatal("Lookup returned nil for registered type")
		}
	})

	t.Run("rejects empty effect type", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()

		err := r.Register("", dummyFactory)
		if err == nil {
			t.Fatal("expected error for empty effect type")
		}
	})

	t.Run("rejects nil factory", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()

		err := r.Register("chorus", nil)
		if err == nil {
			t.Fatal("expected error for nil factory")
		}
	})

	t.Run("rejects duplicate registration", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()
		_ = r.Register("chorus", dummyFactory)

		err := r.Register("chorus", dummyFactory)
		if err == nil {
			t.Fatal("expected error for duplicate registration")
		}

		if !errors.Is(err, errDuplicateEffect) {
			t.Errorf("expected errDuplicateEffect, got: %v", err)
		}
	})
}

func TestRegistryLookup(t *testing.T) {
	t.Parallel()

	t.Run("returns nil for unknown type", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()

		if f := r.Lookup("nonexistent"); f != nil {
			t.Fatal("expected nil for unknown type")
		}
	})

	t.Run("returns nil for empty string", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()

		if f := r.Lookup(""); f != nil {
			t.Fatal("expected nil for empty string")
		}
	})
}

func TestRegistryMustRegister(t *testing.T) {
	t.Parallel()

	t.Run("succeeds for valid registration", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()
		// Should not panic.
		r.MustRegister("chorus", dummyFactory)

		if r.Lookup("chorus") == nil {
			t.Fatal("expected factory after MustRegister")
		}
	})

	t.Run("panics on duplicate", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry()
		r.MustRegister("chorus", dummyFactory)

		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic on duplicate MustRegister")
			}
		}()

		r.MustRegister("chorus", dummyFactory)
	})
}
