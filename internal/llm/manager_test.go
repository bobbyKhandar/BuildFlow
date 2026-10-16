package llm

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"buildflow/internal/appdata"
)

type rejectTransport struct{ calls int }

func (r *rejectTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r.calls++
	panic("summary must not contact a provider with incomplete or paid settings")
}

func TestCompleteFreeRejectsPaidAndIncompleteSettings(t *testing.T) {
	t.Setenv(appdata.EnvOverride, t.TempDir())
	store, err := appdata.Open()
	if err != nil {
		t.Fatal(err)
	}
	m := New(store)
	transport := &rejectTransport{}
	m.client = &http.Client{Transport: transport}
	for _, tc := range []struct{ mode, provider, model, want string }{
		{"premium", "openai", "configured-model", "free-mode settings"},
		{"free", "groq", "", "choose a model"},
		{"free", "groq", "configured-model", "API key"},
	} {
		if err := m.SetConfig(tc.mode, tc.provider, tc.model); err != nil {
			t.Fatal(err)
		}
		_, err := m.CompleteFree(context.Background(), []Message{{Role: "user", Content: "Summarize saved blocks"}})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("mode %s: got %v, want %q", tc.mode, err, tc.want)
		}
		if got := m.Config(); got.Mode != tc.mode || got.Provider != tc.provider || got.Model != tc.model {
			t.Fatalf("completion changed settings: %+v", got)
		}
	}
	if transport.calls != 0 {
		t.Fatalf("unexpected requests: %d", transport.calls)
	}
}
