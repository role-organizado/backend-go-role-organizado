// Package sqs provides the real AWS SQS adapter for outbound notifications.
// It is wired only when cfg.SQS.Enabled is true; otherwise the no-op convite
// notification adapter (mongodb.NoopConviteNotificationAdapter) is used.
package sqs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	appconfig "github.com/role-organizado/backend-go-role-organizado/internal/config"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// sqsAPI is the narrow slice of the SQS client used by the publisher. Declared
// as an interface so tests can substitute a fake without real AWS credentials.
type sqsAPI interface {
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

// ConvitePublisher is the real SQS-backed implementation of
// portout.ConviteNotificationPort. It serialises the convite payload to JSON and
// publishes it to the configured queue (adding a MessageGroupId for FIFO queues).
type ConvitePublisher struct {
	client   sqsAPI
	queueURL string
}

// NewConvitePublisher builds an SQS publisher from the application config. It
// loads the default AWS credential chain (env, shared config, IAM role) for the
// configured region. Returns an error when the queue URL is empty or the AWS
// config cannot be resolved.
func NewConvitePublisher(ctx context.Context, cfg appconfig.SQSConfig) (*ConvitePublisher, error) {
	if strings.TrimSpace(cfg.QueueURL) == "" {
		return nil, fmt.Errorf("sqs: queue URL is empty (set ROLE_SQS_QUEUE_URL)")
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("sqs: load aws config: %w", err)
	}
	return &ConvitePublisher{
		client:   sqs.NewFromConfig(awsCfg),
		queueURL: cfg.QueueURL,
	}, nil
}

// convitePayload is the JSON envelope published to the queue. Mirrors the Java
// SqsNotificationService convite message shape (type + flattened convite data).
type convitePayload struct {
	Type            string     `json:"type"`
	ParticipantID   string     `json:"participantId"`
	Canal           string     `json:"canal,omitempty"`
	Telefone        string     `json:"telefone,omitempty"`
	Email           string     `json:"email,omitempty"`
	Nome            string     `json:"nome,omitempty"`
	EventoID        string     `json:"eventoId,omitempty"`
	EventoNome      string     `json:"eventoNome,omitempty"`
	EventoData      *time.Time `json:"eventoData,omitempty"`
	EventoLocal     string     `json:"eventoLocal,omitempty"`
	OrganizadorNome string     `json:"organizadorNome,omitempty"`
}

// PublicarConvite implements portout.ConviteNotificationPort. It returns the SQS
// message id assigned by AWS on success.
func (p *ConvitePublisher) PublicarConvite(ctx context.Context, in portout.ConvitePublishInput) (string, error) {
	body, err := json.Marshal(convitePayload{
		Type:            "CONVITE",
		ParticipantID:   in.ParticipantID,
		Canal:           in.Canal,
		Telefone:        in.Telefone,
		Email:           in.Email,
		Nome:            in.Nome,
		EventoID:        in.EventoID,
		EventoNome:      in.EventoNome,
		EventoData:      in.EventoData,
		EventoLocal:     in.EventoLocal,
		OrganizadorNome: in.OrganizadorNome,
	})
	if err != nil {
		return "", fmt.Errorf("sqs: marshal convite payload: %w", err)
	}

	input := &sqs.SendMessageInput{
		QueueUrl:    &p.queueURL,
		MessageBody: ptr(string(body)),
	}
	// FIFO queues require a message group id; use a stable "invites" group like Java.
	if isFifoQueue(p.queueURL) {
		input.MessageGroupId = ptr("invites")
		if in.ParticipantID != "" {
			input.MessageDeduplicationId = ptr(in.ParticipantID)
		}
	}

	out, err := p.client.SendMessage(ctx, input)
	if err != nil {
		return "", fmt.Errorf("sqs: send convite message: %w", err)
	}
	if out.MessageId == nil {
		return "", nil
	}
	return *out.MessageId, nil
}

// isFifoQueue reports whether the queue URL targets a FIFO queue (.fifo suffix).
func isFifoQueue(queueURL string) bool {
	return strings.Contains(strings.ToLower(queueURL), ".fifo")
}

func ptr(s string) *string { return &s }

// Compile-time assertion that ConvitePublisher satisfies the output port.
var _ portout.ConviteNotificationPort = (*ConvitePublisher)(nil)
