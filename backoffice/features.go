package backoffice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/AltSoyuz/lib/audit"
	"github.com/AltSoyuz/lib/httpserver"
	"github.com/AltSoyuz/lib/logger"
)

// Audiences of a feature: everyone, or the admins only, as for a beta.
const (
	AudienceEveryone = "everyone"
	AudienceAdmins   = "admins"
)

var (
	errUnknownFeature  = errors.New("unknown_feature")
	errInvalidAudience = errors.New("invalid_audience")
)

// Feature is a part of the app an admin can keep to the admins; the app's UI hides it from everyone else.
// Name is the key the UI tests, Label what the back office shows.
type Feature struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// FeatureState is a feature with its audience: AudienceEveryone until an admin chooses.
type FeatureState struct {
	Feature
	Audience string `json:"audience"`
}

func checkFeatures(features []Feature) error {
	seen := map[string]bool{}
	for _, f := range features {
		if !validFeatureName(f.Name) || f.Label == "" || seen[f.Name] {
			return fmt.Errorf("backoffice: Config.Features has an empty, duplicate or invalid feature %+v (names are a-z, 0-9 and -)", f)
		}
		seen[f.Name] = true
	}
	return nil
}

func validFeatureName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// Features returns the features of Config.Features in their order, with their audience. An app that declares none
// reads nothing, so it needs no features table.
func (b *Backoffice) Features(ctx context.Context) ([]FeatureState, error) {
	out := make([]FeatureState, 0, len(b.cfg.Features))
	if len(b.cfg.Features) == 0 {
		return out, nil
	}
	rows, err := b.cfg.DB.QueryContext(ctx, `SELECT name, audience FROM features`)
	if err != nil {
		return nil, fmt.Errorf("cannot read the features: %w", err)
	}
	defer func() { _ = rows.Close() }()
	stored := map[string]string{}
	for rows.Next() {
		var name, audience string
		if err := rows.Scan(&name, &audience); err != nil {
			return nil, fmt.Errorf("cannot read a feature: %w", err)
		}
		stored[name] = audience
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("cannot read the features: %w", err)
	}
	for _, f := range b.cfg.Features {
		out = append(out, FeatureState{Feature: f, Audience: cmpOr(stored[f.Name], AudienceEveryone)})
	}
	return out, nil
}

// handleFeatures needs no session: an app used signed out hides an admins feature too.
func (b *Backoffice) handleFeatures(w http.ResponseWriter, r *http.Request) {
	features, err := b.Features(r.Context())
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpserver.WriteJSON(w, r, http.StatusOK, map[string]any{"features": features})
}

func (b *Backoffice) handleSetFeature(w http.ResponseWriter, r *http.Request) {
	admin, ok := b.RequireAdmin(w, r)
	if !ok {
		return
	}
	var feature *Feature
	for i, f := range b.cfg.Features {
		if f.Name == r.PathValue("name") {
			feature = &b.cfg.Features[i]
		}
	}
	if feature == nil {
		httpserver.WriteError(w, r, http.StatusNotFound, errUnknownFeature)
		return
	}
	in, err := httpserver.DecodeJSON[struct {
		Audience string `json:"audience"`
	}](r)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, err)
		return
	}
	if in.Audience != AudienceEveryone && in.Audience != AudienceAdmins {
		httpserver.WriteError(w, r, http.StatusBadRequest, errInvalidAudience)
		return
	}
	err = b.inTx(r.Context(), func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO features (name, audience, updated_at, updated_by) VALUES (?, ?, ?, ?)
			ON CONFLICT (name) DO UPDATE SET audience = excluded.audience, updated_at = excluded.updated_at, updated_by = excluded.updated_by`,
			feature.Name, in.Audience, time.Now().UTC().Format(time.RFC3339), admin.UserID); err != nil {
			return fmt.Errorf("cannot save the feature %s: %w", feature.Name, err)
		}
		return audit.Write(r.Context(), tx, admin.UserID, audit.FeatureSet, feature.Name, map[string]any{"label": feature.Label, "audience": in.Audience})
	})
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	logger.Info("feature_set", "by", admin.UserID, "feature", feature.Name, "audience", in.Audience)
	httpserver.WriteJSON(w, r, http.StatusOK, FeatureState{Feature: *feature, Audience: in.Audience})
}
