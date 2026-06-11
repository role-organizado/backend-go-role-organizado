// Package outbound implements the Outbound Transfers domain — supplier
// payments and expense reimbursements with a multi-organizer/voting approval
// flow, mirroring the Java OutboundRequest entity and business rules.
package outbound

import (
	"errors"
	"math"
	"regexp"
	"strings"
	"time"
)

// OutboundStatus is the lifecycle state of an outbound request.
type OutboundStatus string

const (
	StatusPending    OutboundStatus = "PENDING"
	StatusApproved   OutboundStatus = "APPROVED"
	StatusRejected   OutboundStatus = "REJECTED"
	StatusProcessing OutboundStatus = "PROCESSING"
	StatusCompleted  OutboundStatus = "COMPLETED"
	StatusFailed     OutboundStatus = "FAILED"
	StatusCancelled  OutboundStatus = "CANCELLED"
	StatusExpired    OutboundStatus = "EXPIRED"
)

// OutboundType is the kind of outbound request.
type OutboundType string

const (
	TypeSupplierPayment       OutboundType = "SUPPLIER_PAYMENT"
	TypeExpenseReimbursement  OutboundType = "EXPENSE_REIMBURSEMENT"
)

// PixKeyType identifies the PIX key format.
type PixKeyType string

const (
	PixKeyTypeCPF    PixKeyType = "CPF"
	PixKeyTypeCNPJ   PixKeyType = "CNPJ"
	PixKeyTypeEmail  PixKeyType = "EMAIL"
	PixKeyTypePhone  PixKeyType = "PHONE"
	PixKeyTypeRandom PixKeyType = "RANDOM"
)

// ApprovalMode describes how a request is approved.
type ApprovalMode string

const (
	ApprovalModeAuto            ApprovalMode = "AUTO"
	ApprovalModeAllParticipants ApprovalMode = "ALL_PARTICIPANTS"
	ApprovalModeQuorum50        ApprovalMode = "QUORUM_50"
)

// VoteValue captures an individual vote.
type VoteValue string

const (
	VoteApprove VoteValue = "APPROVE"
	VoteReject  VoteValue = "REJECT"
)

// Recipient holds the destination of an outbound transfer.
type Recipient struct {
	Name       string
	Document   string
	PixKey     string
	PixKeyType PixKeyType
}

// Attachment is the proof attached to an EXPENSE_REIMBURSEMENT request.
type Attachment struct {
	ID          string
	Filename    string
	MimeType    string
	Size        int64
	UploadedAt  time.Time
	// URL is transient — populated when the request is read for HTTP responses.
	URL string
}

// Vote is an individual organizer/participant vote on a request.
type Vote struct {
	UserID  string
	Vote    VoteValue
	VotedAt time.Time
	Comment string
}

// OutboundRequest is the central entity of the outbound domain.
// All monetary amounts are stored as int64 cents.
type OutboundRequest struct {
	ID               string
	EventID          string
	RequesterUserID  string
	RateioID         string
	RateioName       string
	Type             OutboundType
	AmountCents      int64
	Justification    string
	PaymentAccountID string // for EXPENSE_REIMBURSEMENT
	Recipient        Recipient
	AttachmentID     string
	Attachment       *Attachment
	Status           OutboundStatus

	// Voting state
	Votes           []Vote
	Approvals       int
	Rejections      int
	RequiredVotes   int
	RequiresVoting  bool
	ApprovalMode    ApprovalMode
	ExpiresAt       *time.Time

	// Approval/Rejection tracking
	ApprovedBy      string
	ApprovedAt      *time.Time
	RejectedBy      string
	RejectedAt      *time.Time
	RejectionReason string

	// Provider tracking
	Provider              string
	ProviderTransferID    string
	FailureReason         string

	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
}

// ── Business methods ─────────────────────────────────────────────────────────

// IsTerminal reports whether the request has reached a final state.
func (r *OutboundRequest) IsTerminal() bool {
	switch r.Status {
	case StatusCompleted, StatusFailed, StatusCancelled, StatusRejected, StatusExpired:
		return true
	}
	return false
}

// CanBeApproved returns true when an organizer can directly approve the request.
func (r *OutboundRequest) CanBeApproved() bool {
	return r.Status == StatusPending
}

// CanBeRejected returns true when an organizer can directly reject the request.
func (r *OutboundRequest) CanBeRejected() bool {
	return r.Status == StatusPending
}

// CanBeCancelled returns true when the requester can still cancel.
func (r *OutboundRequest) CanBeCancelled() bool {
	return r.Status == StatusPending
}

// RequiresAttachment reports whether the request type mandates an attachment.
func (r *OutboundRequest) RequiresAttachment() bool {
	return r.Type == TypeExpenseReimbursement
}

// HasValidAttachment reports whether an attachment is present and valid.
func (r *OutboundRequest) HasValidAttachment() bool {
	return r.AttachmentID != "" || r.Attachment != nil
}

// HasUserVoted reports whether userID already voted on this request.
func (r *OutboundRequest) HasUserVoted(userID string) bool {
	for _, v := range r.Votes {
		if v.UserID == userID {
			return true
		}
	}
	return false
}

// IsVotingExpired returns true if a voting deadline exists and has passed.
func (r *OutboundRequest) IsVotingExpired(now time.Time) bool {
	return r.ExpiresAt != nil && now.After(*r.ExpiresAt)
}

// ConfigureApproval sets the approval mode and required vote count based on the
// number of eligible voters (excluding the requester). Mirrors Java
// configureApproval(eligibleVoters, votingWindowDays).
//
// Rules:
//   - 0 eligible voters → AUTO (requiresVoting=false; will be auto-APPROVED by caller)
//   - 1..4 eligible voters → ALL_PARTICIPANTS (every vote required)
//   - 5+ eligible voters → QUORUM_50 (ceil(eligible/2))
func (r *OutboundRequest) ConfigureApproval(eligibleVoters int, votingWindowDays int) {
	if eligibleVoters <= 0 {
		r.ApprovalMode = ApprovalModeAuto
		r.RequiresVoting = false
		r.RequiredVotes = 0
		r.ExpiresAt = nil
		return
	}

	if eligibleVoters <= 4 {
		r.ApprovalMode = ApprovalModeAllParticipants
		r.RequiredVotes = eligibleVoters
	} else {
		r.ApprovalMode = ApprovalModeQuorum50
		r.RequiredVotes = int(math.Ceil(float64(eligibleVoters) / 2.0))
	}
	r.RequiresVoting = true
	exp := time.Now().Add(time.Duration(votingWindowDays) * 24 * time.Hour)
	r.ExpiresAt = &exp
}

// AddVote appends a vote and updates the running tally.
func (r *OutboundRequest) AddVote(userID string, value VoteValue, comment string) error {
	if r.HasUserVoted(userID) {
		return errors.New("usuário já votou nesta solicitação")
	}
	now := time.Now().UTC()
	r.Votes = append(r.Votes, Vote{
		UserID:  userID,
		Vote:    value,
		VotedAt: now,
		Comment: comment,
	})
	if value == VoteApprove {
		r.Approvals++
	} else {
		r.Rejections++
	}
	r.UpdatedAt = now
	return nil
}

// HasReachedApprovalQuorum reports whether the running tally already satisfies
// the configured approval mode (used by VoteOnOutboundRequest).
func (r *OutboundRequest) HasReachedApprovalQuorum() bool {
	switch r.ApprovalMode {
	case ApprovalModeAllParticipants:
		return r.Approvals >= r.RequiredVotes
	case ApprovalModeQuorum50:
		total := r.Approvals + r.Rejections
		return total >= r.RequiredVotes && r.Approvals > r.Rejections
	}
	return false
}

// HasReachedRejectionQuorum reports whether the running tally already implies
// the request must be rejected.
func (r *OutboundRequest) HasReachedRejectionQuorum() bool {
	switch r.ApprovalMode {
	case ApprovalModeAllParticipants:
		// Any single rejection is enough in ALL_PARTICIPANTS mode.
		return r.Rejections > 0
	case ApprovalModeQuorum50:
		total := r.Approvals + r.Rejections
		return total >= r.RequiredVotes && r.Rejections >= r.Approvals
	}
	return false
}

// Approve transitions PENDING → APPROVED with the given approver and notes.
func (r *OutboundRequest) Approve(approverID, notes string) error {
	if !r.CanBeApproved() {
		return errors.New("apenas solicitações PENDING podem ser aprovadas")
	}
	now := time.Now().UTC()
	r.Status = StatusApproved
	r.ApprovedBy = approverID
	r.ApprovedAt = &now
	r.UpdatedAt = now
	if notes != "" && r.Justification == "" {
		r.Justification = notes
	}
	return nil
}

// Reject transitions PENDING → REJECTED with the given rejecter and reason.
func (r *OutboundRequest) Reject(rejecterID, reason string) error {
	if !r.CanBeRejected() {
		return errors.New("apenas solicitações PENDING podem ser rejeitadas")
	}
	if strings.TrimSpace(reason) == "" {
		return errors.New("motivo da rejeição é obrigatório")
	}
	now := time.Now().UTC()
	r.Status = StatusRejected
	r.RejectedBy = rejecterID
	r.RejectedAt = &now
	r.RejectionReason = reason
	r.UpdatedAt = now
	return nil
}

// RejectImmediately transitions to REJECTED bypassing the PENDING precondition.
// Used by the voting logic when a rejection quorum is met.
func (r *OutboundRequest) RejectImmediately(reason string) {
	now := time.Now().UTC()
	r.Status = StatusRejected
	r.RejectedAt = &now
	r.RejectionReason = reason
	r.UpdatedAt = now
}

// Cancel transitions PENDING → CANCELLED with an optional reason stored in
// RejectionReason for audit symmetry.
func (r *OutboundRequest) Cancel(reason string) error {
	if !r.CanBeCancelled() {
		return errors.New("apenas solicitações PENDING podem ser canceladas")
	}
	now := time.Now().UTC()
	r.Status = StatusCancelled
	r.RejectionReason = reason
	r.UpdatedAt = now
	return nil
}

// MarkAsProcessing transitions APPROVED → PROCESSING.
func (r *OutboundRequest) MarkAsProcessing(provider, providerTransferID string) {
	now := time.Now().UTC()
	r.Status = StatusProcessing
	r.Provider = provider
	r.ProviderTransferID = providerTransferID
	r.UpdatedAt = now
}

// MarkAsCompleted transitions PROCESSING → COMPLETED.
func (r *OutboundRequest) MarkAsCompleted() {
	now := time.Now().UTC()
	r.Status = StatusCompleted
	r.CompletedAt = &now
	r.UpdatedAt = now
}

// MarkAsFailed transitions to FAILED.
func (r *OutboundRequest) MarkAsFailed(reason string) {
	now := time.Now().UTC()
	r.Status = StatusFailed
	r.FailureReason = reason
	r.UpdatedAt = now
}

// MarkAsExpired transitions to EXPIRED when the voting window passes without a
// decision.
func (r *OutboundRequest) MarkAsExpired() {
	now := time.Now().UTC()
	r.Status = StatusExpired
	r.UpdatedAt = now
}

// ── PIX key validation ───────────────────────────────────────────────────────

var (
	rePixCPF    = regexp.MustCompile(`^\d{11}$`)
	rePixCNPJ   = regexp.MustCompile(`^\d{14}$`)
	rePixEmail  = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	rePixPhone  = regexp.MustCompile(`^\+55\d{10,11}$`)
	rePixRandom = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
)

// ValidatePixKey returns an error if the given key does not match the format
// expected for kind. Mirrors the Java validation rules.
func ValidatePixKey(kind PixKeyType, key string) error {
	key = strings.TrimSpace(key)
	switch kind {
	case PixKeyTypeCPF:
		if !rePixCPF.MatchString(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(key, ".", ""), "-", ""), "/", "")) {
			return errors.New("chave PIX CPF deve conter 11 dígitos")
		}
	case PixKeyTypeCNPJ:
		clean := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(key, ".", ""), "-", ""), "/", "")
		if !rePixCNPJ.MatchString(clean) {
			return errors.New("chave PIX CNPJ deve conter 14 dígitos")
		}
	case PixKeyTypeEmail:
		if !rePixEmail.MatchString(key) {
			return errors.New("chave PIX EMAIL inválida")
		}
	case PixKeyTypePhone:
		clean := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(key, " ", ""), "(", ""), ")", "")
		clean = strings.ReplaceAll(clean, "-", "")
		if !rePixPhone.MatchString(clean) {
			return errors.New("chave PIX PHONE deve estar no formato +55DDDNUMERO")
		}
	case PixKeyTypeRandom:
		if !rePixRandom.MatchString(key) {
			return errors.New("chave PIX RANDOM deve ser UUID v4")
		}
	default:
		return errors.New("tipo de chave PIX inválido")
	}
	return nil
}
