package notification_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notifdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
	paymentdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	ucnotification "github.com/role-organizado/backend-go-role-organizado/internal/usecase/notification"
)

// ─── Fake repo ───────────────────────────────────────────────────────────────

type fakeWebhookRepo struct {
	existsFn func(ctx context.Context, provider, eventID string) (bool, error)
	saveFn   func(ctx context.Context, e *paymentdomain.ProcessedWebhookEvent) error

	existsCalls []string
	saved       []paymentdomain.ProcessedWebhookEvent
}

func (r *fakeWebhookRepo) ExistsByProviderAndEventID(ctx context.Context, provider, eventID string) (bool, error) {
	r.existsCalls = append(r.existsCalls, provider+":"+eventID)
	if r.existsFn != nil {
		return r.existsFn(ctx, provider, eventID)
	}
	return false, nil
}

func (r *fakeWebhookRepo) SaveUnique(ctx context.Context, e *paymentdomain.ProcessedWebhookEvent) error {
	if r.saveFn != nil {
		return r.saveFn(ctx, e)
	}
	r.saved = append(r.saved, *e)
	return nil
}

var _ portout.ProcessedWebhookEventRepository = (*fakeWebhookRepo)(nil)

// ─── Helpers ─────────────────────────────────────────────────────────────────

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// ─── Tests ────────────────────────────────────────────────────────────────────

// TestHandleWhatsAppWebhook_StatusEvent_Persists verifies the happy path: a
// single status event triggers a dedup check + persist with the canonical
// eventID format "<entryId>:<id>:status" and eventType "STATUS_<UPPER>".
func TestHandleWhatsAppWebhook_StatusEvent_Persists(t *testing.T) {
	repo := &fakeWebhookRepo{}
	clock := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	uc := ucnotification.NewHandleWhatsAppWebhook(repo).WithClock(fixedClock(clock))

	payload := notifdomain.WhatsAppWebhookPayload{
		Object: "whatsapp_business_account",
		Entry: []notifdomain.WhatsAppWebhookEntry{{
			ID: "waba_123",
			Changes: []notifdomain.WhatsAppWebhookChange{{
				Field: "messages",
				Value: notifdomain.WhatsAppWebhookValue{
					Statuses: []notifdomain.WhatsAppWebhookStatus{{
						ID:     "wamid.ABC",
						Status: "delivered",
					}},
				},
			}},
		}},
	}

	require.NoError(t, uc.Execute(context.Background(), payload))
	require.Len(t, repo.saved, 1)
	saved := repo.saved[0]
	assert.Equal(t, notifdomain.WhatsAppProvider, saved.Provider)
	assert.Equal(t, "waba_123:wamid.ABC:status", saved.EventID)
	assert.Equal(t, "wamid.ABC", saved.ProviderTransactionID)
	assert.Equal(t, "STATUS_DELIVERED", saved.EventType)
	assert.Equal(t, clock, saved.ProcessedAt)
}

// TestHandleWhatsAppWebhook_MessageEvent_Persists covers the inbound message
// branch with the MESSAGE_<TYPE> naming convention.
func TestHandleWhatsAppWebhook_MessageEvent_Persists(t *testing.T) {
	repo := &fakeWebhookRepo{}
	uc := ucnotification.NewHandleWhatsAppWebhook(repo)

	payload := notifdomain.WhatsAppWebhookPayload{
		Object: "whatsapp_business_account",
		Entry: []notifdomain.WhatsAppWebhookEntry{{
			ID: "waba_42",
			Changes: []notifdomain.WhatsAppWebhookChange{{
				Field: "messages",
				Value: notifdomain.WhatsAppWebhookValue{
					Messages: []notifdomain.WhatsAppWebhookMessage{{
						ID:   "wamid.MSG",
						Type: "text",
						From: "5511999999999",
					}},
				},
			}},
		}},
	}

	require.NoError(t, uc.Execute(context.Background(), payload))
	require.Len(t, repo.saved, 1)
	assert.Equal(t, "waba_42:wamid.MSG:message", repo.saved[0].EventID)
	assert.Equal(t, "MESSAGE_TEXT", repo.saved[0].EventType)
}

// TestHandleWhatsAppWebhook_DuplicateEvent_SkipsSave verifies that when the
// dedup check reports the event is already processed, no persistence call is
// made and Execute still returns nil.
func TestHandleWhatsAppWebhook_DuplicateEvent_SkipsSave(t *testing.T) {
	repo := &fakeWebhookRepo{
		existsFn: func(ctx context.Context, provider, eventID string) (bool, error) {
			return true, nil
		},
	}
	uc := ucnotification.NewHandleWhatsAppWebhook(repo)

	payload := notifdomain.WhatsAppWebhookPayload{
		Object: "whatsapp_business_account",
		Entry: []notifdomain.WhatsAppWebhookEntry{{
			ID: "waba",
			Changes: []notifdomain.WhatsAppWebhookChange{{
				Value: notifdomain.WhatsAppWebhookValue{
					Statuses: []notifdomain.WhatsAppWebhookStatus{{ID: "wamid.X", Status: "read"}},
				},
			}},
		}},
	}

	require.NoError(t, uc.Execute(context.Background(), payload))
	assert.Empty(t, repo.saved, "duplicate must not be persisted")
	assert.Len(t, repo.existsCalls, 1)
}

// TestHandleWhatsAppWebhook_RaceDuplicate_Swallows verifies that an
// ErrAlreadyProcessed from SaveUnique (race between Exists check and write)
// is swallowed — Execute still returns nil so the HTTP layer can reply 200.
func TestHandleWhatsAppWebhook_RaceDuplicate_Swallows(t *testing.T) {
	repo := &fakeWebhookRepo{
		saveFn: func(ctx context.Context, e *paymentdomain.ProcessedWebhookEvent) error {
			return portout.ErrAlreadyProcessed
		},
	}
	uc := ucnotification.NewHandleWhatsAppWebhook(repo)

	payload := notifdomain.WhatsAppWebhookPayload{
		Object: "whatsapp_business_account",
		Entry: []notifdomain.WhatsAppWebhookEntry{{
			ID: "waba",
			Changes: []notifdomain.WhatsAppWebhookChange{{
				Value: notifdomain.WhatsAppWebhookValue{
					Statuses: []notifdomain.WhatsAppWebhookStatus{{ID: "wamid.R", Status: "sent"}},
				},
			}},
		}},
	}

	assert.NoError(t, uc.Execute(context.Background(), payload))
}

// TestHandleWhatsAppWebhook_UnsupportedObject_Ignored verifies that when the
// top-level object isn't whatsapp_business_account, the repo is never touched.
func TestHandleWhatsAppWebhook_UnsupportedObject_Ignored(t *testing.T) {
	repo := &fakeWebhookRepo{}
	uc := ucnotification.NewHandleWhatsAppWebhook(repo)

	require.NoError(t, uc.Execute(context.Background(), notifdomain.WhatsAppWebhookPayload{
		Object: "instagram",
		Entry: []notifdomain.WhatsAppWebhookEntry{{
			ID: "x",
			Changes: []notifdomain.WhatsAppWebhookChange{{
				Value: notifdomain.WhatsAppWebhookValue{
					Statuses: []notifdomain.WhatsAppWebhookStatus{{ID: "y", Status: "delivered"}},
				},
			}},
		}},
	}))
	assert.Empty(t, repo.saved)
	assert.Empty(t, repo.existsCalls)
}

// TestHandleWhatsAppWebhook_EmptyEntry_Ignored verifies the Meta verification
// ping pattern (object set, entry empty) is treated as a no-op.
func TestHandleWhatsAppWebhook_EmptyEntry_Ignored(t *testing.T) {
	repo := &fakeWebhookRepo{}
	uc := ucnotification.NewHandleWhatsAppWebhook(repo)

	require.NoError(t, uc.Execute(context.Background(), notifdomain.WhatsAppWebhookPayload{
		Object: "whatsapp_business_account",
		Entry:  nil,
	}))
	assert.Empty(t, repo.saved)
	assert.Empty(t, repo.existsCalls)
}

// TestHandleWhatsAppWebhook_MissingMessageID_Skipped covers the malformed
// payload branch: events without an id can't be deduplicated, so we skip them
// (and log a warning) instead of persisting a meaningless row.
func TestHandleWhatsAppWebhook_MissingMessageID_Skipped(t *testing.T) {
	repo := &fakeWebhookRepo{}
	uc := ucnotification.NewHandleWhatsAppWebhook(repo)

	require.NoError(t, uc.Execute(context.Background(), notifdomain.WhatsAppWebhookPayload{
		Object: "whatsapp_business_account",
		Entry: []notifdomain.WhatsAppWebhookEntry{{
			ID: "waba",
			Changes: []notifdomain.WhatsAppWebhookChange{{
				Value: notifdomain.WhatsAppWebhookValue{
					Messages: []notifdomain.WhatsAppWebhookMessage{{ID: "", Type: "text"}},
					Statuses: []notifdomain.WhatsAppWebhookStatus{{ID: "", Status: "read"}},
				},
			}},
		}},
	}))
	assert.Empty(t, repo.saved)
	assert.Empty(t, repo.existsCalls, "missing-id events must not even hit the dedup check")
}

// TestHandleWhatsAppWebhook_ExistsError_Swallowed verifies that a dedup-check
// failure does not abort processing of other events in the same payload.
func TestHandleWhatsAppWebhook_ExistsError_Swallowed(t *testing.T) {
	repo := &fakeWebhookRepo{
		existsFn: func(ctx context.Context, provider, eventID string) (bool, error) {
			return false, errors.New("transient mongo error")
		},
	}
	uc := ucnotification.NewHandleWhatsAppWebhook(repo)

	assert.NoError(t, uc.Execute(context.Background(), notifdomain.WhatsAppWebhookPayload{
		Object: "whatsapp_business_account",
		Entry: []notifdomain.WhatsAppWebhookEntry{{
			ID: "waba",
			Changes: []notifdomain.WhatsAppWebhookChange{{
				Value: notifdomain.WhatsAppWebhookValue{
					Statuses: []notifdomain.WhatsAppWebhookStatus{{ID: "wamid.A", Status: "delivered"}},
				},
			}},
		}},
	}))
}
