package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/TimurManjosov/goflagship/internal/engine"
	"github.com/TimurManjosov/goflagship/internal/rules"
	"github.com/TimurManjosov/goflagship/internal/snapshot"
	"github.com/TimurManjosov/goflagship/internal/store"
)

func TestHandleContextEvaluate(t *testing.T) {
	setupEvaluationSnapshot([]store.Flag{
		{
			Key:     "beta",
			Enabled: true,
			Config:  map[string]any{"global": true},
			Variants: []store.Variant{
				{Name: "control", Weight: 50, Config: map[string]any{"color": "blue"}},
				{Name: "treatment", Weight: 50, Config: map[string]any{"color": "red"}},
			},
			TargetingRules: []rules.Rule{
				{
					ID: "country-us",
					Conditions: []rules.Condition{
						{Property: "country", Operator: rules.OpEq, Value: "US"},
					},
				},
			},
		},
		{Key: "alpha", Enabled: true, Config: map[string]any{"alpha": true}},
		{Key: "off", Enabled: false, Config: map[string]any{"off": true}},
	})

	handler := NewServer(store.NewMemoryStore(), "prod", "test-key").Router()

	testCases := []struct {
		name           string
		body           string
		expectedStatus int
		assert         func(t *testing.T, rr *httptest.ResponseRecorder)
	}{
		{
			name:           "specific flag key returns single result",
			body:           `{"context":{"id":"user-1","country":"US"},"flagKey":"beta"}`,
			expectedStatus: http.StatusOK,
			assert: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp EvaluationResponse
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if len(resp.Results) != 1 {
					t.Fatalf("expected 1 result, got %d", len(resp.Results))
				}
				if resp.Results[0].Key != "beta" {
					t.Fatalf("expected key beta, got %s", resp.Results[0].Key)
				}
			},
		},
		{
			name:           "empty flag key evaluates all flags in deterministic order",
			body:           `{"context":{"id":"user-1","country":"US"}}`,
			expectedStatus: http.StatusOK,
			assert: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp EvaluationResponse
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if len(resp.Results) != 3 {
					t.Fatalf("expected 3 results, got %d", len(resp.Results))
				}
				if resp.Results[0].Key != "alpha" || resp.Results[1].Key != "beta" || resp.Results[2].Key != "off" {
					t.Fatalf("unexpected key ordering: %+v", resp.Results)
				}
			},
		},
		{
			name:           "unknown flag key returns 404",
			body:           `{"context":{"id":"user-1"},"flagKey":"unknown"}`,
			expectedStatus: http.StatusNotFound,
			assert: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp ErrorResponse
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if resp.Code != ErrCodeNotFound {
					t.Fatalf("expected NOT_FOUND code, got %s", resp.Code)
				}
			},
		},
		{
			name:           "malformed json returns 400",
			body:           `{"context":{"id":"user-1"`,
			expectedStatus: http.StatusBadRequest,
			assert: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp ErrorResponse
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if resp.Code != ErrCodeInvalidJSON {
					t.Fatalf("expected INVALID_JSON code, got %s", resp.Code)
				}
			},
		},
		{
			name:           "reason variant and value are propagated",
			body:           `{"context":{"id":"user-1","country":"US"},"flagKey":"beta"}`,
			expectedStatus: http.StatusOK,
			assert: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp EvaluationResponse
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				result := resp.Results[0]
				if result.Reason != string(engine.ReasonTargetingMatch) {
					t.Fatalf("expected reason %s, got %s", engine.ReasonTargetingMatch, result.Reason)
				}
				if result.Variant == "" {
					t.Fatal("expected non-empty variant")
				}
				value, ok := result.Value.(map[string]any)
				if !ok {
					t.Fatalf("expected value to be object, got %T", result.Value)
				}
				if _, ok := value["color"]; !ok {
					t.Fatalf("expected variant value payload, got %+v", value)
				}
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
			tt.assert(t, rr)
		})
	}
}

func TestHandleContextEvaluate_RequestTooLarge(t *testing.T) {
	setupEvaluationSnapshot([]store.Flag{{Key: "alpha", Enabled: true}})
	handler := NewServer(store.NewMemoryStore(), "prod", "test-key").Router()

	largeContext := strings.Repeat("a", (1<<20)+1)
	body := `{"context":{"id":"user-1","properties":{"blob":"` + largeContext + `"}}}`

	req := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleContextEvaluate_UsesSnapshotWithoutStoreCalls(t *testing.T) {
	setupEvaluationSnapshot([]store.Flag{{Key: "alpha", Enabled: true}})
	handler := NewServer(panicStore{}, "prod", "test-key").Router()

	req := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewBufferString(`{"context":{"id":"user-1"}}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleContextEvaluate_ConcurrentSnapshotReads(t *testing.T) {
	setupEvaluationSnapshot([]store.Flag{
		{Key: "alpha", Enabled: true},
		{Key: "beta", Enabled: true},
	})
	handler := NewServer(store.NewMemoryStore(), "prod", "test-key").Router()

	var wg sync.WaitGroup
	errCh := make(chan string, 40)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewBufferString(`{"context":{"id":"user-1"}}`))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				errCh <- rr.Body.String()
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("unexpected non-200 response: %s", err)
	}
}

func setupEvaluationSnapshot(flags []store.Flag) {
	snapshot.SetRolloutSalt("test-salt")
	snapshot.Update(snapshot.BuildFromFlags(flags))
}

type panicStore struct{}

func (panicStore) GetAllFlags(context.Context, string) ([]store.Flag, error) {
	panic("GetAllFlags should not be called")
}

func (panicStore) GetFlagByKey(context.Context, string) (*store.Flag, error) {
	panic("GetFlagByKey should not be called")
}

func (panicStore) UpsertFlag(context.Context, store.UpsertParams) error {
	panic("UpsertFlag should not be called")
}

func (panicStore) DeleteFlag(context.Context, string, string) error {
	panic("DeleteFlag should not be called")
}

func (panicStore) Close() error {
	return nil
}
