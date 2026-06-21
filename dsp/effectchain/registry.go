package effectchain

import (
	"errors"
	"fmt"
)

// Factory builds one Runtime instance for a node.
type Factory func(ctx Context) (Runtime, error)

// Registry maps effect type names to their factories.
type Registry struct {
	factories map[string]Factory
}

var errDuplicateEffect = errors.New("duplicate effect type")

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

// Register adds a factory for the given effect type.
func (r *Registry) Register(effectType string, factory Factory) error {
	if effectType == "" {
		return errors.New("empty effect type")
	}

	if factory == nil {
		return errors.New("nil factory")
	}

	if _, exists := r.factories[effectType]; exists {
		return fmt.Errorf("%w: %s", errDuplicateEffect, effectType)
	}

	r.factories[effectType] = factory

	return nil
}

// MustRegister is like Register but panics on error.
func (r *Registry) MustRegister(effectType string, factory Factory) {
	err := r.Register(effectType, factory)
	if err != nil {
		panic("effectchain registry: " + err.Error())
	}
}

// Lookup returns the factory for the given effect type, or nil.
func (r *Registry) Lookup(effectType string) Factory {
	return r.factories[effectType]
}
