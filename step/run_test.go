package step

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/inngest/inngest/pkg/enums"
	"github.com/inngest/inngest/pkg/execution/state"
	"github.com/khulnasoft-lab/inngestgo/internal/sdkrequest"
	"github.com/stretchr/testify/require"
)

func TestStep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	req := &sdkrequest.Request{
		Steps: map[string]json.RawMessage{},
	}
	mgr := sdkrequest.NewManager(cancel, req)
	ctx = sdkrequest.SetManager(ctx, mgr)

	type response struct {
		OK       bool           `json:"ok"`
		SomeData map[string]any `json:"someData"`
	}

	expected := response{
		OK: true,
		SomeData: map[string]any{
			"what": "is",
			// NOTE: Unmarshalling this input data always returns a float due to
			// the JSON representation
			"life": float64(42),
		},
	}

	opData, err := json.Marshal(map[string]any{"data": expected})
	require.NoError(t, err)

	name := "My test step"

	t.Run("Step state", func(t *testing.T) {
		t.Run("Struct values", func(t *testing.T) {
			// Construct an op outside of the manager so that we don't mess with
			// indexes
			name = "struct"
			op := sdkrequest.UnhashedOp{
				Op: enums.OpcodeStep,
				ID: name,
			}

			req.Steps[op.MustHash()] = opData
			val, err := Run(ctx, name, func(ctx context.Context) (response, error) {
				// memoized state, return doesnt matter
				return response{}, nil
			})
			require.NoError(t, err)
			require.Equal(t, expected, val)
			require.Empty(t, mgr.Ops())
		})

		t.Run("Struct pointers", func(t *testing.T) {
			// Construct an op outside of the manager so that we don't mess with
			// indexes
			name = "struct ptrs"
			op := sdkrequest.UnhashedOp{
				Op: enums.OpcodeStep,
				ID: name,
			}

			req.Steps[op.MustHash()] = opData
			val, err := Run(ctx, name, func(ctx context.Context) (*response, error) {
				// memoized state, return doesnt matter
				return nil, nil
			})
			require.NoError(t, err)
			require.EqualValues(t, &expected, val)
			require.Empty(t, mgr.Ops())
		})

		t.Run("Slices", func(t *testing.T) {
			t.Run("With wrapped 'data' field", func(t *testing.T) {
				// Construct an op outside of the manager so that we don't mess with
				// indexes
				name = "slices-data"
				op := sdkrequest.UnhashedOp{
					Op: enums.OpcodeStep,
					ID: name,
				}

				byt, err := json.Marshal(map[string]any{
					"data": []response{expected},
				})
				require.NoError(t, err)
				req.Steps[op.MustHash()] = byt

				val, err := Run(ctx, name, func(ctx context.Context) ([]response, error) {
					// memoized state, return doesnt matter
					return nil, nil
				})
				require.NoError(t, err)
				require.EqualValues(t, []response{expected}, val)
				require.Empty(t, mgr.Ops())
			})

			t.Run("With raw data in op", func(t *testing.T) {
				// Construct an op outside of the manager so that we don't mess with
				// indexes
				name = "slices-raw"
				op := sdkrequest.UnhashedOp{
					Op: enums.OpcodeStep,
					ID: name,
				}

				byt, err := json.Marshal([]response{expected})
				require.NoError(t, err)
				req.Steps[op.MustHash()] = byt

				val, err := Run(ctx, name, func(ctx context.Context) ([]response, error) {
					// memoized state, return doesnt matter
					return nil, nil
				})
				require.NoError(t, err)
				require.EqualValues(t, []response{expected}, val)
				require.Empty(t, mgr.Ops())
			})
		})

		t.Run("Ints", func(t *testing.T) {
			// Construct an op outside of the manager so that we don't mess with
			// indexes
			name = "ints"
			op := sdkrequest.UnhashedOp{
				Op: enums.OpcodeStep,
				ID: name,
			}

			// Add a new number
			byt, err := json.Marshal(map[string]any{"data": 646})
			require.NoError(t, err)
			req.Steps[op.MustHash()] = byt

			val, err := Run(ctx, name, func(ctx context.Context) (int, error) {
				// memoized state, return doesnt matter
				return 0, nil
			})
			require.NoError(t, err)
			require.EqualValues(t, 646, val)
			require.Empty(t, mgr.Ops())
		})

		t.Run("nil", func(t *testing.T) {
			// Construct an op outside of the manager so that we don't mess with
			// indexes
			name = "nil"
			op := sdkrequest.UnhashedOp{
				Op: enums.OpcodeStep,
				ID: name,
			}

			// Add nil
			opData, err := json.Marshal(nil)
			require.NoError(t, err)
			req.Steps[op.MustHash()] = opData

			val, err := Run(ctx, name, func(ctx context.Context) (any, error) {
				// memoized state, return doesnt matter
				return nil, nil
			})
			require.NoError(t, err)
			require.EqualValues(t, nil, val)
			require.Empty(t, mgr.Ops())
		})
	})

	t.Run("No state", func(t *testing.T) {
		t.Run("Appends opcodes", func(t *testing.T) {
			name = "new step must append"

			func() {
				defer func() {
					rcv := recover()
					require.Equal(t, ControlHijack{}, rcv)
				}()

				_, err := Run(ctx, name, func(ctx context.Context) (response, error) {
					return expected, nil
				})
				require.NoError(t, err)
			}()

			op := sdkrequest.UnhashedOp{
				Op: enums.OpcodeStep,
				ID: name,
			}

			require.NotEmpty(t, mgr.Ops())
			require.Equal(t, 1, len(mgr.Ops()))
			require.Equal(t, []state.GeneratorOpcode{{
				ID:   op.MustHash(),
				Op:   enums.OpcodeStep,
				Name: name,
				Data: opData,
			}}, mgr.Ops())
		})
	})

	t.Run("It doesn't do anything with a cancelled context", func(t *testing.T) {
		mgr.Cancel()

		func() {
			defer func() {
				rcv := recover()
				require.Equal(t, ControlHijack{}, rcv)
			}()
			val, err := Run(ctx, "new", func(ctx context.Context) (response, error) {
				return expected, nil
			})
			require.NoError(t, err)
			require.Empty(t, val)
		}()
	})
}
