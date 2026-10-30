package backoffice

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
)

func (a *app) features() []FeatureState {
	a.t.Helper()
	w := a.call("GET", "/api/v1/features", nil)
	if w.Code != http.StatusOK {
		a.t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body)
	}
	var out struct {
		Features []FeatureState `json:"features"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		a.t.Fatalf("decode %s: %v", w.Body, err)
	}
	return out.Features
}

func audiences(features []FeatureState) string {
	out := ""
	for _, f := range features {
		out += f.Name + ":" + f.Audience + " "
	}
	return out
}

func TestFeaturesAreForEveryoneUntilAnAdminKeepsOneToTheAdmins(t *testing.T) {
	a := newApp(t, false)
	admin, _ := a.signIn(superAdmin)
	member, _ := a.signIn("member@example.com")
	if got := audiences(a.features()); got != "events:everyone skillbooks:everyone " {
		t.Fatalf("expected every feature for everyone, got %s", got)
	}
	body := map[string]string{"audience": AudienceAdmins}
	if w := a.call("PUT", "/api/v1/admin/features/events", body, member); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a member, got %d body=%s", w.Code, w.Body)
	}
	if w := a.call("PUT", "/api/v1/admin/features/events", body); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 signed out, got %d body=%s", w.Code, w.Body)
	}
	if w := a.call("PUT", "/api/v1/admin/features/chat", body, admin); w.Code != http.StatusNotFound || errorOf(w) != "unknown_feature" {
		t.Fatalf("expected 404 unknown_feature, got %d body=%s", w.Code, w.Body)
	}
	if w := a.call("PUT", "/api/v1/admin/features/events", map[string]string{"audience": "nobody"}, admin); w.Code != http.StatusBadRequest || errorOf(w) != "invalid_audience" {
		t.Fatalf("expected 400 invalid_audience, got %d body=%s", w.Code, w.Body)
	}
	w := a.call("PUT", "/api/v1/admin/features/events", body, admin)
	var saved FeatureState
	if err := json.Unmarshal(w.Body.Bytes(), &saved); w.Code != http.StatusOK || err != nil || saved.Label != "Events" || saved.Audience != AudienceAdmins {
		t.Fatalf("expected 200 with events for the admins, got %d body=%s", w.Code, w.Body)
	}
	if got := audiences(a.features()); got != "events:admins skillbooks:everyone " {
		t.Fatalf("expected events for the admins only, got %s", got)
	}
	if got := a.actions(); got[len(got)-1] != "feature_set" {
		t.Fatalf("expected a feature_set audit row last, got %v", got)
	}
	if got := a.bo.actions(); !slices.Contains(got, "feature_set") {
		t.Fatalf("expected feature_set in the audit filter of an app with features, got %v", got)
	}
	if w := a.call("PUT", "/api/v1/admin/features/events", map[string]string{"audience": AudienceEveryone}, admin); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body)
	}
	if got := audiences(a.features()); got != "events:everyone skillbooks:everyone " {
		t.Fatalf("expected events back for everyone, got %s", got)
	}
}

func TestAnAppWithoutFeaturesNeedsNoFeaturesTable(t *testing.T) {
	a := newApp(t, false)
	if _, err := a.db.Exec(`DROP TABLE features`); err != nil {
		t.Fatal(err)
	}
	bo, err := New(Config{DB: a.db, Roles: []string{RoleAdmin, "user"}, MemberRole: "user", Sessions: testPolicy})
	if err != nil {
		t.Fatal(err)
	}
	got, err := bo.Features(context.Background())
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("expected an empty list without the table, got %v, %v", got, err)
	}
	if slices.Contains(bo.actions(), "feature_set") {
		t.Fatalf("expected no feature_set in the audit filter of an app without features, got %v", bo.actions())
	}
}

func TestNewRefusesAnInvalidFeature(t *testing.T) {
	a := newApp(t, false)
	for _, features := range [][]Feature{
		{{Name: "", Label: "Events"}},
		{{Name: "events", Label: ""}},
		{{Name: "Events", Label: "Events"}},
		{{Name: "live events", Label: "Events"}},
		{{Name: "events", Label: "Events"}, {Name: "events", Label: "Events again"}},
	} {
		if _, err := New(Config{DB: a.db, Roles: []string{RoleAdmin, "user"}, MemberRole: "user", Sessions: testPolicy, Features: features}); err == nil {
			t.Fatalf("expected New to refuse %+v", features)
		}
	}
}
