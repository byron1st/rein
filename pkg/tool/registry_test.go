package tool_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/byron1st/rein/pkg/tool"
	"github.com/stretchr/testify/require"
)

type fakeTool struct {
	name    string
	schema  json.RawMessage
	out     string
	err     error
	gotArgs json.RawMessage
}

func (f *fakeTool) Name() string            { return f.name }
func (f *fakeTool) Schema() json.RawMessage { return f.schema }
func (f *fakeTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	f.gotArgs = args
	return f.out, f.err
}

func TestRegister_Get_List_Dispatch(t *testing.T) {
	reg := tool.NewRegistry()
	alpha := &fakeTool{name: "alpha", schema: json.RawMessage(`{"name":"alpha"}`), out: "alpha-out"}
	beta := &fakeTool{name: "beta", schema: json.RawMessage(`{"name":"beta"}`), out: "beta-out"}
	reg.Register(alpha)
	reg.Register(beta)

	t.Run("Get returns registered tool", func(t *testing.T) {
		got, ok := reg.Get("alpha")
		require.True(t, ok, "expected alpha to be registered")
		require.Equal(t, alpha, got)
	})

	t.Run("Get missing returns false", func(t *testing.T) {
		got, ok := reg.Get("missing")
		require.False(t, ok, "expected missing tool to be absent")
		require.Nil(t, got)
	})

	t.Run("List preserves registration order", func(t *testing.T) {
		list := reg.List()
		require.Len(t, list, 2)
		require.Equal(t, "alpha", list[0].Name(), "expected alpha first in registration order")
		require.Equal(t, "beta", list[1].Name(), "expected beta second in registration order")
	})

	t.Run("Register overwrite keeps existing order position", func(t *testing.T) {
		reg := tool.NewRegistry()
		reg.Register(&fakeTool{name: "x", out: "old"})
		reg.Register(&fakeTool{name: "y", out: "y"})
		reg.Register(&fakeTool{name: "x", out: "new"})

		list := reg.List()
		require.Len(t, list, 2, "overwrite must not duplicate order entries")
		require.Equal(t, "x", list[0].Name(), "x must keep its original position")
		require.Equal(t, "y", list[1].Name())

		got, ok := reg.Get("x")
		require.True(t, ok)
		require.Equal(t, "new", got.(*fakeTool).out, "map entry must be the overwritten tool")
	})

	t.Run("Dispatch returns tool output and forwards args", func(t *testing.T) {
		args := json.RawMessage(`{"x":1}`)
		out, err := reg.Dispatch(context.Background(), "alpha", args)
		require.NoError(t, err)
		require.Equal(t, "alpha-out", out)
		require.JSONEq(t, `{"x":1}`, string(alpha.gotArgs), "Dispatch must forward args verbatim")
	})
}

func TestDispatch_UnknownTool_ReturnsErrUnknownTool(t *testing.T) {
	reg := tool.NewRegistry()
	_, err := reg.Dispatch(context.Background(), "nope", json.RawMessage(`{}`))
	require.ErrorIs(t, err, tool.ErrUnknownTool)
	require.Contains(t, err.Error(), "nope", "error must name the unknown tool")
}

func TestDispatch_PropagatesToolError(t *testing.T) {
	errBoom := errors.New("boom")
	reg := tool.NewRegistry()
	reg.Register(&fakeTool{name: "boom-tool", err: errBoom})

	_, err := reg.Dispatch(context.Background(), "boom-tool", json.RawMessage(`{}`))
	require.ErrorIs(t, err, errBoom, "Dispatch must propagate the tool's error unchanged")
	require.NotErrorIs(t, err, tool.ErrUnknownTool, "Dispatch must not re-wrap tool errors")
}

func TestList_EmptyRegistry_ReturnsNonNilEmptySlice(t *testing.T) {
	reg := tool.NewRegistry()
	list := reg.List()
	require.NotNil(t, list, "List must return a non-nil slice when empty")
	require.Len(t, list, 0)
}
