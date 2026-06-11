package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// Compile-time assertions.
var (
	_ portout.OutboundRequestRepository  = (*OutboundRequestMongoRepository)(nil)
	_ portout.OutboundAuditLogRepository = (*OutboundAuditLogMongoRepository)(nil)
)

const (
	colOutboundRequests  = "outbound_requests"
	colOutboundAuditLogs = "outbound_audit_logs"
)

// ─── Documents ───────────────────────────────────────────────────────────────

type outboundRecipientDoc struct {
	Name       string `bson:"name,omitempty"`
	Document   string `bson:"document,omitempty"`
	PixKey     string `bson:"pix_key,omitempty"`
	PixKeyType string `bson:"pix_key_type,omitempty"`
}

type outboundAttachmentDoc struct {
	ID         string    `bson:"id,omitempty"`
	Filename   string    `bson:"filename,omitempty"`
	MimeType   string    `bson:"mime_type,omitempty"`
	Size       int64     `bson:"size,omitempty"`
	UploadedAt time.Time `bson:"uploaded_at,omitempty"`
}

type outboundVoteDoc struct {
	UserID  string    `bson:"user_id"`
	Vote    string    `bson:"vote"`
	VotedAt time.Time `bson:"voted_at"`
	Comment string    `bson:"comment,omitempty"`
}

type outboundRequestDoc struct {
	ID                 string                 `bson:"_id,omitempty"`
	EventID            string                 `bson:"event_id"`
	RequesterUserID    string                 `bson:"requester_user_id"`
	RateioID           string                 `bson:"rateio_id,omitempty"`
	RateioName         string                 `bson:"rateio_name,omitempty"`
	Type               string                 `bson:"type"`
	AmountCents        int64                  `bson:"amount_cents"`
	Justification      string                 `bson:"justification,omitempty"`
	PaymentAccountID   string                 `bson:"payment_account_id,omitempty"`
	Recipient          outboundRecipientDoc   `bson:"recipient,omitempty"`
	AttachmentID       string                 `bson:"attachment_id,omitempty"`
	Attachment         *outboundAttachmentDoc `bson:"attachment,omitempty"`
	Status             string                 `bson:"status"`
	Votes              []outboundVoteDoc      `bson:"votes,omitempty"`
	Approvals          int                    `bson:"approvals,omitempty"`
	Rejections         int                    `bson:"rejections,omitempty"`
	RequiredVotes      int                    `bson:"required_votes,omitempty"`
	RequiresVoting     bool                   `bson:"requires_voting,omitempty"`
	ApprovalMode       string                 `bson:"approval_mode,omitempty"`
	ExpiresAt          *time.Time             `bson:"expires_at,omitempty"`
	ApprovedBy         string                 `bson:"approved_by,omitempty"`
	ApprovedAt         *time.Time             `bson:"approved_at,omitempty"`
	RejectedBy         string                 `bson:"rejected_by,omitempty"`
	RejectedAt         *time.Time             `bson:"rejected_at,omitempty"`
	RejectionReason    string                 `bson:"rejection_reason,omitempty"`
	Provider           string                 `bson:"provider,omitempty"`
	ProviderTransferID string                 `bson:"provider_transfer_id,omitempty"`
	FailureReason      string                 `bson:"failure_reason,omitempty"`
	CreatedAt          time.Time              `bson:"created_at"`
	UpdatedAt          time.Time              `bson:"updated_at"`
	CompletedAt        *time.Time             `bson:"completed_at,omitempty"`
}

// ─── OutboundRequestMongoRepository ──────────────────────────────────────────

// OutboundRequestMongoRepository persists OutboundRequest documents to MongoDB.
type OutboundRequestMongoRepository struct {
	col *mongo.Collection
}

// NewOutboundRequestRepository creates a new repository wired to the
// outbound_requests collection.
func NewOutboundRequestRepository(client *Client) *OutboundRequestMongoRepository {
	return &OutboundRequestMongoRepository{col: client.Collection(colOutboundRequests)}
}

// Save inserts a new request, generating a UUID for ID if absent.
func (r *OutboundRequestMongoRepository) Save(ctx context.Context, req *domain.OutboundRequest) (*domain.OutboundRequest, error) {
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now().UTC()
	}
	req.UpdatedAt = time.Now().UTC()
	doc := outboundReqToDoc(req)
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return nil, apierr.Internal(fmt.Sprintf("save outbound request: %s", err.Error()))
	}
	return req, nil
}

// Update replaces an existing document.
func (r *OutboundRequestMongoRepository) Update(ctx context.Context, req *domain.OutboundRequest) (*domain.OutboundRequest, error) {
	req.UpdatedAt = time.Now().UTC()
	doc := outboundReqToDoc(req)
	res, err := r.col.ReplaceOne(ctx, bson.D{{Key: "_id", Value: req.ID}}, doc)
	if err != nil {
		return nil, apierr.Internal(fmt.Sprintf("update outbound request: %s", err.Error()))
	}
	if res.MatchedCount == 0 {
		return nil, apierr.NotFound("outbound_request", req.ID)
	}
	return req, nil
}

// FindByID returns a request by its platform ID.
func (r *OutboundRequestMongoRepository) FindByID(ctx context.Context, id string) (*domain.OutboundRequest, error) {
	var doc outboundRequestDoc
	if err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("outbound_request", id)
		}
		return nil, apierr.Internal(fmt.Sprintf("find outbound request by id: %s", err.Error()))
	}
	req := outboundReqFromDoc(doc)
	return &req, nil
}

// FindByIDAndEventID enforces eventID scoping.
func (r *OutboundRequestMongoRepository) FindByIDAndEventID(ctx context.Context, id, eventID string) (*domain.OutboundRequest, error) {
	var doc outboundRequestDoc
	filter := bson.D{{Key: "_id", Value: id}, {Key: "event_id", Value: eventID}}
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("outbound_request", id)
		}
		return nil, apierr.Internal(fmt.Sprintf("find outbound request by id and event: %s", err.Error()))
	}
	req := outboundReqFromDoc(doc)
	return &req, nil
}

func (r *OutboundRequestMongoRepository) findMany(ctx context.Context, filter bson.D) ([]domain.OutboundRequest, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, apierr.Internal(fmt.Sprintf("find outbound requests: %s", err.Error()))
	}
	defer cursor.Close(ctx)
	var results []domain.OutboundRequest
	for cursor.Next(ctx) {
		var doc outboundRequestDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, apierr.Internal(fmt.Sprintf("decode outbound request: %s", err.Error()))
		}
		results = append(results, outboundReqFromDoc(doc))
	}
	if err := cursor.Err(); err != nil {
		return nil, apierr.Internal(fmt.Sprintf("cursor outbound requests: %s", err.Error()))
	}
	if results == nil {
		results = []domain.OutboundRequest{}
	}
	return results, nil
}

// FindByEventID returns all requests for an event.
func (r *OutboundRequestMongoRepository) FindByEventID(ctx context.Context, eventID string) ([]domain.OutboundRequest, error) {
	return r.findMany(ctx, bson.D{{Key: "event_id", Value: eventID}})
}

// FindByEventIDAndStatus filters by status.
func (r *OutboundRequestMongoRepository) FindByEventIDAndStatus(ctx context.Context, eventID string, status domain.OutboundStatus) ([]domain.OutboundRequest, error) {
	return r.findMany(ctx, bson.D{{Key: "event_id", Value: eventID}, {Key: "status", Value: string(status)}})
}

// FindByEventIDAndType filters by type.
func (r *OutboundRequestMongoRepository) FindByEventIDAndType(ctx context.Context, eventID string, t domain.OutboundType) ([]domain.OutboundRequest, error) {
	return r.findMany(ctx, bson.D{{Key: "event_id", Value: eventID}, {Key: "type", Value: string(t)}})
}

// FindByRequesterUserID returns all requests made by a given user.
func (r *OutboundRequestMongoRepository) FindByRequesterUserID(ctx context.Context, userID string) ([]domain.OutboundRequest, error) {
	return r.findMany(ctx, bson.D{{Key: "requester_user_id", Value: userID}})
}

// FindPendingByEventID returns all PENDING requests for an event.
func (r *OutboundRequestMongoRepository) FindPendingByEventID(ctx context.Context, eventID string) ([]domain.OutboundRequest, error) {
	return r.FindByEventIDAndStatus(ctx, eventID, domain.StatusPending)
}

// CountPendingByEventID counts pending requests for an event.
func (r *OutboundRequestMongoRepository) CountPendingByEventID(ctx context.Context, eventID string) (int64, error) {
	count, err := r.col.CountDocuments(ctx, bson.D{
		{Key: "event_id", Value: eventID},
		{Key: "status", Value: string(domain.StatusPending)},
	})
	if err != nil {
		return 0, apierr.Internal(fmt.Sprintf("count pending outbound requests: %s", err.Error()))
	}
	return count, nil
}

// ExistsActiveByRateioID returns true if any PENDING/APPROVED/PROCESSING outbound
// request exists for the rateio. Used to block duplicate active outbounds.
func (r *OutboundRequestMongoRepository) ExistsActiveByRateioID(ctx context.Context, rateioID string) (bool, error) {
	if rateioID == "" {
		return false, nil
	}
	active := []string{string(domain.StatusPending), string(domain.StatusApproved), string(domain.StatusProcessing)}
	count, err := r.col.CountDocuments(ctx, bson.D{
		{Key: "rateio_id", Value: rateioID},
		{Key: "status", Value: bson.D{{Key: "$in", Value: active}}},
	}, options.Count().SetLimit(1))
	if err != nil {
		return false, apierr.Internal(fmt.Sprintf("check active outbound by rateio: %s", err.Error()))
	}
	return count > 0, nil
}

// FindByProviderTransferID resolves a webhook callback to its request.
func (r *OutboundRequestMongoRepository) FindByProviderTransferID(ctx context.Context, providerTransferID string) (*domain.OutboundRequest, error) {
	var doc outboundRequestDoc
	filter := bson.D{{Key: "provider_transfer_id", Value: providerTransferID}}
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("outbound_request", providerTransferID)
		}
		return nil, apierr.Internal(fmt.Sprintf("find outbound by provider transfer id: %s", err.Error()))
	}
	req := outboundReqFromDoc(doc)
	return &req, nil
}

// DeleteByID removes a request (currently unused by the UC layer; kept for parity).
func (r *OutboundRequestMongoRepository) DeleteByID(ctx context.Context, id string) error {
	res, err := r.col.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	if err != nil {
		return apierr.Internal(fmt.Sprintf("delete outbound request: %s", err.Error()))
	}
	if res.DeletedCount == 0 {
		return apierr.NotFound("outbound_request", id)
	}
	return nil
}

// ─── OutboundAuditLogMongoRepository ─────────────────────────────────────────

// OutboundAuditLogMongoRepository persists outbound audit-log entries.
type OutboundAuditLogMongoRepository struct {
	col *mongo.Collection
}

// NewOutboundAuditLogRepository creates a new audit-log repository.
func NewOutboundAuditLogRepository(client *Client) *OutboundAuditLogMongoRepository {
	return &OutboundAuditLogMongoRepository{col: client.Collection(colOutboundAuditLogs)}
}

type outboundAuditLogDoc struct {
	ID         string    `bson:"_id,omitempty"`
	RequestID  string    `bson:"request_id"`
	EventID    string    `bson:"event_id,omitempty"`
	ActorID    string    `bson:"actor_id,omitempty"`
	Action     string    `bson:"action"`
	Details    string    `bson:"details,omitempty"`
	OccurredAt time.Time `bson:"occurred_at"`
}

// Append inserts a new audit entry.
func (r *OutboundAuditLogMongoRepository) Append(ctx context.Context, entry *domain.AuditLog) error {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.OccurredAt.IsZero() {
		entry.OccurredAt = time.Now().UTC()
	}
	doc := outboundAuditLogDoc{
		ID:         entry.ID,
		RequestID:  entry.RequestID,
		EventID:    entry.EventID,
		ActorID:    entry.ActorID,
		Action:     string(entry.Action),
		Details:    entry.Details,
		OccurredAt: entry.OccurredAt,
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return apierr.Internal(fmt.Sprintf("append outbound audit log: %s", err.Error()))
	}
	return nil
}

// FindByRequestID lists the audit trail for a request.
func (r *OutboundAuditLogMongoRepository) FindByRequestID(ctx context.Context, requestID string) ([]domain.AuditLog, error) {
	cursor, err := r.col.Find(ctx,
		bson.D{{Key: "request_id", Value: requestID}},
		options.Find().SetSort(bson.D{{Key: "occurred_at", Value: 1}}),
	)
	if err != nil {
		return nil, apierr.Internal(fmt.Sprintf("find outbound audit logs: %s", err.Error()))
	}
	defer cursor.Close(ctx)

	var results []domain.AuditLog
	for cursor.Next(ctx) {
		var doc outboundAuditLogDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, apierr.Internal(fmt.Sprintf("decode outbound audit log: %s", err.Error()))
		}
		results = append(results, domain.AuditLog{
			ID:         doc.ID,
			RequestID:  doc.RequestID,
			EventID:    doc.EventID,
			ActorID:    doc.ActorID,
			Action:     domain.AuditAction(doc.Action),
			Details:    doc.Details,
			OccurredAt: doc.OccurredAt,
		})
	}
	if results == nil {
		results = []domain.AuditLog{}
	}
	return results, nil
}

// ─── Mapping helpers ─────────────────────────────────────────────────────────

func outboundReqToDoc(r *domain.OutboundRequest) outboundRequestDoc {
	doc := outboundRequestDoc{
		ID:                 r.ID,
		EventID:            r.EventID,
		RequesterUserID:    r.RequesterUserID,
		RateioID:           r.RateioID,
		RateioName:         r.RateioName,
		Type:               string(r.Type),
		AmountCents:        r.AmountCents,
		Justification:      r.Justification,
		PaymentAccountID:   r.PaymentAccountID,
		Recipient: outboundRecipientDoc{
			Name:       r.Recipient.Name,
			Document:   r.Recipient.Document,
			PixKey:     r.Recipient.PixKey,
			PixKeyType: string(r.Recipient.PixKeyType),
		},
		AttachmentID:       r.AttachmentID,
		Status:             string(r.Status),
		Approvals:          r.Approvals,
		Rejections:         r.Rejections,
		RequiredVotes:      r.RequiredVotes,
		RequiresVoting:     r.RequiresVoting,
		ApprovalMode:       string(r.ApprovalMode),
		ExpiresAt:          r.ExpiresAt,
		ApprovedBy:         r.ApprovedBy,
		ApprovedAt:         r.ApprovedAt,
		RejectedBy:         r.RejectedBy,
		RejectedAt:         r.RejectedAt,
		RejectionReason:    r.RejectionReason,
		Provider:           r.Provider,
		ProviderTransferID: r.ProviderTransferID,
		FailureReason:      r.FailureReason,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
		CompletedAt:        r.CompletedAt,
	}
	if r.Attachment != nil {
		doc.Attachment = &outboundAttachmentDoc{
			ID:         r.Attachment.ID,
			Filename:   r.Attachment.Filename,
			MimeType:   r.Attachment.MimeType,
			Size:       r.Attachment.Size,
			UploadedAt: r.Attachment.UploadedAt,
		}
	}
	if len(r.Votes) > 0 {
		doc.Votes = make([]outboundVoteDoc, len(r.Votes))
		for i, v := range r.Votes {
			doc.Votes[i] = outboundVoteDoc{
				UserID:  v.UserID,
				Vote:    string(v.Vote),
				VotedAt: v.VotedAt,
				Comment: v.Comment,
			}
		}
	}
	return doc
}

func outboundReqFromDoc(doc outboundRequestDoc) domain.OutboundRequest {
	req := domain.OutboundRequest{
		ID:                 doc.ID,
		EventID:            doc.EventID,
		RequesterUserID:    doc.RequesterUserID,
		RateioID:           doc.RateioID,
		RateioName:         doc.RateioName,
		Type:               domain.OutboundType(doc.Type),
		AmountCents:        doc.AmountCents,
		Justification:      doc.Justification,
		PaymentAccountID:   doc.PaymentAccountID,
		Recipient: domain.Recipient{
			Name:       doc.Recipient.Name,
			Document:   doc.Recipient.Document,
			PixKey:     doc.Recipient.PixKey,
			PixKeyType: domain.PixKeyType(doc.Recipient.PixKeyType),
		},
		AttachmentID:       doc.AttachmentID,
		Status:             domain.OutboundStatus(doc.Status),
		Approvals:          doc.Approvals,
		Rejections:         doc.Rejections,
		RequiredVotes:      doc.RequiredVotes,
		RequiresVoting:     doc.RequiresVoting,
		ApprovalMode:       domain.ApprovalMode(doc.ApprovalMode),
		ExpiresAt:          doc.ExpiresAt,
		ApprovedBy:         doc.ApprovedBy,
		ApprovedAt:         doc.ApprovedAt,
		RejectedBy:         doc.RejectedBy,
		RejectedAt:         doc.RejectedAt,
		RejectionReason:    doc.RejectionReason,
		Provider:           doc.Provider,
		ProviderTransferID: doc.ProviderTransferID,
		FailureReason:      doc.FailureReason,
		CreatedAt:          doc.CreatedAt,
		UpdatedAt:          doc.UpdatedAt,
		CompletedAt:        doc.CompletedAt,
	}
	if doc.Attachment != nil {
		req.Attachment = &domain.Attachment{
			ID:         doc.Attachment.ID,
			Filename:   doc.Attachment.Filename,
			MimeType:   doc.Attachment.MimeType,
			Size:       doc.Attachment.Size,
			UploadedAt: doc.Attachment.UploadedAt,
		}
	}
	if len(doc.Votes) > 0 {
		req.Votes = make([]domain.Vote, len(doc.Votes))
		for i, v := range doc.Votes {
			req.Votes[i] = domain.Vote{
				UserID:  v.UserID,
				Vote:    domain.VoteValue(v.Vote),
				VotedAt: v.VotedAt,
				Comment: v.Comment,
			}
		}
	}
	return req
}
