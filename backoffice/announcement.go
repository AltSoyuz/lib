package backoffice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AltSoyuz/lib/audit"
	"github.com/AltSoyuz/lib/httpserver"
	"github.com/AltSoyuz/lib/logger"
)

const (
	announcementMaxChars = 280
	toneInfo             = "info"
	toneWarning          = "warning"
	// announcementLayout is fixed width: updated_at values compare as strings and a phone keys the banner it closed by it.
	announcementLayout = "2006-01-02T15:04:05.000000000Z"
)

var (
	errInvalidAnnouncementText = errors.New("invalid_text")
	errInvalidAnnouncementTone = errors.New("invalid_tone")
)

// Announcement is the banner an admin shows to every active account.
type Announcement struct {
	Text      string `json:"text"`
	Tone      string `json:"tone"`
	UpdatedAt string `json:"updated_at"`
}

// ActiveAnnouncement returns the announcement shown now, or nil when there is none.
func (b *Backoffice) ActiveAnnouncement(ctx context.Context) (*Announcement, error) {
	var a Announcement
	err := b.cfg.DB.QueryRowContext(ctx, `SELECT text, tone, updated_at FROM announcement WHERE id = 1 AND active = 1`).Scan(&a.Text, &a.Tone, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read the announcement: %w", err)
	}
	return &a, nil
}

func (b *Backoffice) handleSetAnnouncement(w http.ResponseWriter, r *http.Request) {
	admin, ok := b.RequireAdmin(w, r)
	if !ok {
		return
	}
	in, err := httpserver.DecodeJSON[struct {
		Text string `json:"text"`
		Tone string `json:"tone"`
	}](r)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, err)
		return
	}
	text := strings.TrimSpace(in.Text)
	chars := utf8.RuneCountInString(text)
	if chars == 0 || chars > announcementMaxChars {
		httpserver.WriteError(w, r, http.StatusBadRequest, errInvalidAnnouncementText)
		return
	}
	if in.Tone != toneInfo && in.Tone != toneWarning {
		httpserver.WriteError(w, r, http.StatusBadRequest, errInvalidAnnouncementTone)
		return
	}
	out := Announcement{Text: text, Tone: in.Tone, UpdatedAt: time.Now().UTC().Format(announcementLayout)}
	err = b.inTx(r.Context(), func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO announcement (id, text, tone, active, updated_at, updated_by) VALUES (1, ?, ?, 1, ?, ?)
			ON CONFLICT (id) DO UPDATE SET text = excluded.text, tone = excluded.tone, active = 1, updated_at = excluded.updated_at, updated_by = excluded.updated_by`,
			out.Text, out.Tone, out.UpdatedAt, admin.UserID); err != nil {
			return fmt.Errorf("cannot save the announcement: %w", err)
		}
		return audit.Write(r.Context(), tx, admin.UserID, audit.AnnouncementSet, "", map[string]any{"tone": out.Tone, "length": chars})
	})
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	logger.Info("announcement_set", "by", admin.UserID, "tone", out.Tone, "length", chars)
	httpserver.WriteJSON(w, r, http.StatusOK, out)
}

// handleClearAnnouncement answers 204 when no announcement is active, without an audit row.
func (b *Backoffice) handleClearAnnouncement(w http.ResponseWriter, r *http.Request) {
	admin, ok := b.RequireAdmin(w, r)
	if !ok {
		return
	}
	cleared := false
	err := b.inTx(r.Context(), func(tx *sql.Tx) error {
		res, err := tx.ExecContext(r.Context(), `UPDATE announcement SET active = 0, updated_at = ?, updated_by = ? WHERE id = 1 AND active = 1`,
			time.Now().UTC().Format(announcementLayout), admin.UserID)
		if err != nil {
			return fmt.Errorf("cannot clear the announcement: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("cannot clear the announcement: %w", err)
		}
		if cleared = n > 0; !cleared {
			return nil
		}
		return audit.Write(r.Context(), tx, admin.UserID, audit.AnnouncementCleared, "", nil)
	})
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	if cleared {
		logger.Info("announcement_cleared", "by", admin.UserID)
	}
	w.WriteHeader(http.StatusNoContent)
}
