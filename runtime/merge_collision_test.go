package defc

import (
	"database/sql"
	"errors"
	"io"
	"log"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type tenantCtxT struct{ ID string }

func (t tenantCtxT) ToNamedArgs() map[string]any {
	return map[string]any{"tenant_id": t.ID}
}

type tenantCtxB struct{ ID string }

func (t tenantCtxB) ToNamedArgs() map[string]any {
	return map[string]any{"tenant_id": t.ID, "role": "admin"}
}

type reqBody struct {
	TenantID string `db:"tenant_id"`
	Name     string `db:"name"`
}

type bothArgs struct{}

func (bothArgs) ToArgs() []any                { return []any{1, 2} }
func (bothArgs) ToNamedArgs() map[string]any  { return map[string]any{"a": 1} }

type innerID struct {
	ID string `db:"tenant_id"`
}

type outerWithEmbedCollision struct {
	innerID
	TenantID string `db:"tenant_id"`
}

type evilNamed struct{}

func (evilNamed) ToNamedArgs() map[string]any { return map[string]any{"tenant_id": "evil"} }

type nilable struct {
	TenantID string `db:"tenant_id"`
}

type emptyTNA struct{}

func (emptyTNA) ToNamedArgs() map[string]any { return nil }

type collisionCase struct {
	name            string
	data            map[string]any
	wantKeys        []string
	wantCollision   bool
	wantSourcesKey  string   // the colliding bind key
	wantSourcesSet  []string // expected provenance labels as a set
}

func silenceLog(t *testing.T) {
	t.Helper()
	orig := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(orig) })
}

func collisionMatrix() []collisionCase {
	return []collisionCase{
		{name: "nil_map", data: map[string]any{"m": (map[string]any)(nil)}, wantKeys: nil},
		{name: "typed_nil_pointer", data: map[string]any{"f": (*nilable)(nil)}, wantKeys: []string{"f"}},
		{name: "plain_map_flatten", data: map[string]any{
			"m": map[string]any{"a": 1, "b": 2, "c": 3},
		}, wantKeys: []string{"a", "b", "c"}},
		{name: "map_vs_struct_collision", data: map[string]any{
			"m":   map[string]any{"tenant_id": "attacker", "extra": 9},
			"req": reqBody{TenantID: "real", Name: "bob"},
		}, wantCollision: true, wantSourcesKey: "tenant_id",
			wantSourcesSet: []string{"map(m)", "struct(req.TenantID)"}},
		{name: "two_to_named_args_collision", data: map[string]any{
			"a": tenantCtxT{ID: "A"},
			"b": tenantCtxB{ID: "B"},
		}, wantCollision: true, wantSourcesKey: "tenant_id",
			wantSourcesSet: []string{"ToNamedArgs(a)", "ToNamedArgs(b)"}},
		{name: "both_to_args_and_to_named_args",
			data:     map[string]any{"x": bothArgs{}},
			wantKeys: []string{"a"}},
		{name: "embed_struct_intra_collision",
			data: map[string]any{"s": outerWithEmbedCollision{
				innerID:  innerID{ID: "inner"},
				TenantID: "outer",
			}},
			wantCollision:  true,
			wantSourcesKey: "tenant_id",
			wantSourcesSet: []string{"struct(s.innerID.ID)", "struct(s.TenantID)"}},
		{name: "nullstring_valuer",
			data:     map[string]any{"name": sql.NullString{String: "x", Valid: true}},
			wantKeys: []string{"name"}},
		{name: "slice_stays_opaque",
			data:     map[string]any{"ids": []int{1, 2, 3}},
			wantKeys: []string{"ids"}},
		{name: "empty_slice", data: map[string]any{"ids": []int{}}, wantKeys: []string{"ids"}},
		{name: "nil_slice",
			data: map[string]any{"ids": []int(nil)}, wantKeys: []string{"ids"}},
		{name: "empty_tna",
			data: map[string]any{"e": emptyTNA{}}, wantKeys: nil},
		{name: "scalar_vs_to_named_collision",
			data: map[string]any{
				"c":         evilNamed{},
				"tenant_id": "authoritative",
			},
			wantCollision:  true,
			wantSourcesKey: "tenant_id",
			wantSourcesSet: []string{"ToNamedArgs(c)", "scalar(tenant_id)"}},
		{name: "int_key_map_dropped",
			data: map[string]any{"weird": map[int]string{1: "a"}}, wantKeys: nil},
		{name: "db_tag_dash",
			data: map[string]any{"s": struct {
				X string `db:"-"`
			}{X: "ignored"}}, wantKeys: nil},
		{name: "ints_no_collision",
			data:     map[string]any{"one": 1, "two": 2, "three": 3},
			wantKeys: []string{"one", "two", "three"}},
	}
}

func TestMergeNamedArgsStrict_Matrix(t *testing.T) {
	silenceLog(t)
	for _, tc := range collisionMatrix() {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MergeNamedArgsStrict(tc.data)
			if tc.wantCollision {
				if err == nil {
					t.Fatalf("expected collision error, got nil")
				}
				if !errors.Is(err, ErrNamedArgsCollision) {
					t.Fatalf("want ErrNamedArgsCollision, got %v", err)
				}
				if got != nil {
					t.Fatalf("expected nil map on error, got %v", got)
				}
				var col *NamedArgsCollisionError
				if !errors.As(err, &col) {
					// Could be aggregate -- search inside
					var agg NamedArgsCollisionErrors
					if errors.As(err, &agg) && len(agg) > 0 {
						col = &agg[0]
					}
				}
				if col == nil {
					t.Fatalf("could not extract NamedArgsCollisionError from %v", err)
				}
				if col.Key != tc.wantSourcesKey {
					t.Fatalf("key %q != %q", col.Key, tc.wantSourcesKey)
				}
				gotSet := append([]string(nil), col.Sources...)
				wantSet := append([]string(nil), tc.wantSourcesSet...)
				sort.Strings(gotSet)
				sort.Strings(wantSet)
				if !reflect.DeepEqual(gotSet, wantSet) {
					t.Fatalf("sources set %v != %v", gotSet, wantSet)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotKeys := keysOf(got)
			if len(gotKeys) == 0 && len(tc.wantKeys) == 0 {
				return
			}
			wantKeys := append([]string(nil), tc.wantKeys...)
			sort.Strings(gotKeys)
			sort.Strings(wantKeys)
			if !reflect.DeepEqual(gotKeys, wantKeys) {
				t.Fatalf("keys %v != %v", gotKeys, wantKeys)
			}
		})
	}
}

func TestMergeNamedArgsStrict_Stability(t *testing.T) {
	silenceLog(t)
	for _, tc := range collisionMatrix() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.wantCollision {
				want := append([]string(nil), tc.wantSourcesSet...)
				sort.Strings(want)
				for i := 0; i < 1000; i++ {
					_, err := MergeNamedArgsStrict(tc.data)
					if !errors.Is(err, ErrNamedArgsCollision) {
						t.Fatalf("iter %d: want collision, got %v", i, err)
					}
					var col *NamedArgsCollisionError
					if !errors.As(err, &col) {
						var agg NamedArgsCollisionErrors
						if errors.As(err, &agg) && len(agg) > 0 {
							col = &agg[0]
						}
					}
					if col == nil || col.Key != tc.wantSourcesKey {
						t.Fatalf("iter %d: bad error %v", i, err)
					}
					gotSet := append([]string(nil), col.Sources...)
					sort.Strings(gotSet)
					if !reflect.DeepEqual(gotSet, want) {
						t.Fatalf("iter %d: sources %v != %v", i, gotSet, want)
					}
				}
				return
			}
			var baseline map[string]any
			for i := 0; i < 100; i++ {
				got, err := MergeNamedArgsStrict(tc.data)
				if err != nil {
					t.Fatalf("iter %d: unexpected err %v", i, err)
				}
				if i == 0 {
					baseline = got
					continue
				}
				if !reflect.DeepEqual(keysSortedOf(got), keysSortedOf(baseline)) {
					t.Fatalf("iter %d: non-deterministic keys %v vs %v",
						i, keysSortedOf(got), keysSortedOf(baseline))
				}
			}
		})
	}
}

func TestMergeNamedArgs_HookAndRateLimit(t *testing.T) {
	silenceLog(t)
	resetMergeCollisionCountersForTest()
	t.Cleanup(func() {
		SetOnMergeCollision(nil)
		SetMergeCollisionRateLimit(100)
		resetMergeCollisionCountersForTest()
	})

	var events atomic.Int64
	var mu sync.Mutex
	var lastEvent MergeCollisionEvent
	SetOnMergeCollision(func(e MergeCollisionEvent) {
		events.Add(1)
		mu.Lock()
		lastEvent = e
		mu.Unlock()
	})

	data := map[string]any{
		"a": tenantCtxT{ID: "A"},
		"b": tenantCtxB{ID: "B"},
	}

	SetMergeCollisionRateLimit(3)
	for i := 0; i < 10; i++ {
		MergeNamedArgs(data)
	}
	if got := events.Load(); got != 3 {
		t.Fatalf("rate-limited events: got %d, want 3", got)
	}
	mu.Lock()
	if lastEvent.Key != "tenant_id" {
		t.Fatalf("hook key: %q", lastEvent.Key)
	}
	mu.Unlock()
}

func TestMergeNamedArgs_HookConcurrent(t *testing.T) {
	silenceLog(t)
	resetMergeCollisionCountersForTest()
	t.Cleanup(func() {
		SetOnMergeCollision(nil)
		SetMergeCollisionRateLimit(100)
		resetMergeCollisionCountersForTest()
	})

	SetMergeCollisionRateLimit(0) // unlimited
	var n atomic.Int64
	SetOnMergeCollision(func(e MergeCollisionEvent) { n.Add(1) })

	data := map[string]any{
		"a": tenantCtxT{ID: "A"},
		"b": tenantCtxB{ID: "B"},
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				MergeNamedArgs(data)
			}
		}()
	}
	wg.Wait()
	if got := n.Load(); got != 1600 {
		t.Fatalf("concurrent hook events: got %d, want 1600", got)
	}
}

func TestMergeNamedArgs_LegacyKeepsKeys(t *testing.T) {
	silenceLog(t)
	for _, tc := range collisionMatrix() {
		tc := tc
		if tc.wantCollision {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			got := MergeNamedArgs(tc.data)
			gotKeys := keysOf(got)
			if len(gotKeys) == 0 && len(tc.wantKeys) == 0 {
				return
			}
			wantKeys := append([]string(nil), tc.wantKeys...)
			sort.Strings(gotKeys)
			sort.Strings(wantKeys)
			if !reflect.DeepEqual(gotKeys, wantKeys) {
				t.Fatalf("keys %v != %v", gotKeys, wantKeys)
			}
		})
	}
}

func TestNamedArgsCollisionError_Format(t *testing.T) {
	e := &NamedArgsCollisionError{
		Key:     "tenant_id",
		Sources: []string{"ToNamedArgs(ctx)", "struct(req.TenantID)"},
	}
	if got := e.Error(); !strings.Contains(got, `"tenant_id"`) ||
		!strings.Contains(got, "ToNamedArgs(ctx) and struct(req.TenantID)") {
		t.Fatalf("unexpected message: %q", got)
	}
	e3 := &NamedArgsCollisionError{Key: "k", Sources: []string{"a", "b", "c"}}
	if got := e3.Error(); !strings.Contains(got, "a, b and c") {
		t.Fatalf("unexpected three-source message: %q", got)
	}
	agg := NamedArgsCollisionErrors{
		{Key: "a", Sources: []string{"s1", "s2"}},
		{Key: "b", Sources: []string{"s3", "s4"}},
	}
	if !errors.Is(agg, ErrNamedArgsCollision) {
		t.Fatalf("aggregate not Is ErrNamedArgsCollision")
	}
	if !strings.Contains(agg.Error(), "; ") {
		t.Fatalf("aggregate Error() missing separator: %q", agg.Error())
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func keysSortedOf(m map[string]any) []string {
	k := keysOf(m)
	sort.Strings(k)
	return k
}
