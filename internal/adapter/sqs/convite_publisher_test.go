package sqs

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

type fakeSQS struct {
	lastInput *sqs.SendMessageInput
	msgID     string
	err       error
}

func (f *fakeSQS) SendMessage(_ context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.lastInput = in
	if f.err != nil {
		return nil, f.err
	}
	return &sqs.SendMessageOutput{MessageId: &f.msgID}, nil
}

func TestPublicarConvite_StandardQueue(t *testing.T) {
	fake := &fakeSQS{msgID: "msg-123"}
	pub := &ConvitePublisher{client: fake, queueURL: "https://sqs.us-east-1.amazonaws.com/1/invites"}

	id, err := pub.PublicarConvite(context.Background(), portout.ConvitePublishInput{
		ParticipantID: "p1",
		Canal:         "EMAIL",
		Email:         "a@b.com",
		EventoNome:    "Festa",
	})
	require.NoError(t, err)
	assert.Equal(t, "msg-123", id)
	require.NotNil(t, fake.lastInput)
	assert.Nil(t, fake.lastInput.MessageGroupId, "standard queue must not set a group id")

	var payload convitePayload
	require.NoError(t, json.Unmarshal([]byte(*fake.lastInput.MessageBody), &payload))
	assert.Equal(t, "CONVITE", payload.Type)
	assert.Equal(t, "p1", payload.ParticipantID)
	assert.Equal(t, "Festa", payload.EventoNome)
}

func TestPublicarConvite_FifoQueueSetsGroupAndDedup(t *testing.T) {
	fake := &fakeSQS{msgID: "msg-9"}
	pub := &ConvitePublisher{client: fake, queueURL: "https://sqs.us-east-1.amazonaws.com/1/invites.fifo"}

	_, err := pub.PublicarConvite(context.Background(), portout.ConvitePublishInput{ParticipantID: "p7"})
	require.NoError(t, err)
	require.NotNil(t, fake.lastInput.MessageGroupId)
	assert.Equal(t, "invites", *fake.lastInput.MessageGroupId)
	require.NotNil(t, fake.lastInput.MessageDeduplicationId)
	assert.Equal(t, "p7", *fake.lastInput.MessageDeduplicationId)
}

func TestPublicarConvite_PropagatesError(t *testing.T) {
	fake := &fakeSQS{err: errors.New("aws down")}
	pub := &ConvitePublisher{client: fake, queueURL: "q"}
	_, err := pub.PublicarConvite(context.Background(), portout.ConvitePublishInput{ParticipantID: "p1"})
	require.Error(t, err)
}

func TestIsFifoQueue(t *testing.T) {
	assert.True(t, isFifoQueue("https://x/invites.fifo"))
	assert.False(t, isFifoQueue("https://x/invites"))
}
