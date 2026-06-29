package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrUnknownTool is returned by Dispatch when the requested tool name is not
// registered.
var ErrUnknownTool = errors.New("unknown tool")

// Registry holds tools keyed by name and preserves registration order so List
// is deterministic (map iteration order is not).
type Registry struct {
	tools map[string]Tool
	order []Tool
}

// NewRegistry returns an empty Registry ready for registration.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register registers or overwrites a tool by its Name. A new name is appended
// to the order slice; an existing name overwrites the map entry but keeps its
// original position in the order slice (it is never duplicated).
func (r *Registry) Register(t Tool) {
	name := t.Name()
	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, t)
	}
	r.tools[name] = t
}

// Get returns the tool registered under name, or (nil, false) if absent.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns a copy of the registered tools in registration order. It returns
// a non-nil empty slice when no tools are registered.
func (r *Registry) List() []Tool {
	out := make([]Tool, len(r.order))
	copy(out, r.order)
	return out
}

// Dispatch looks up the tool by name and runs its Execute. An unknown name
// yields an error joining ErrUnknownTool with the name. A known tool's Execute
// result (including its error) is returned unchanged — the tool already owns its
// own error sentinel, so Dispatch does not re-wrap it.
func (r *Registry) Dispatch(ctx context.Context, name string, args json.RawMessage) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", errors.Join(ErrUnknownTool, fmt.Errorf("tool %q not registered", name))
	}
	return t.Execute(ctx, args)
}
