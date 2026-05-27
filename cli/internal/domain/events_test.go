package domain_test

import (
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

func TestEmitMessage_SendsToChannel(t *testing.T) {
	// Initialize the channel
	ch := make(chan domain.DomainMessage, 5)
	domain.DomainEventChan = ch
	defer func() {
		domain.DomainEventChan = nil
	}()

	// Emit messages
	domain.EmitMessage(domain.MsgInfo, "test info")
	domain.EmitMessage(domain.MsgWarning, "test warning")

	// Read and verify
	msg1 := <-ch
	if msg1.Type != domain.MsgInfo || msg1.Message != "test info" {
		t.Errorf("expected info message, got %+v", msg1)
	}

	msg2 := <-ch
	if msg2.Type != domain.MsgWarning || msg2.Message != "test warning" {
		t.Errorf("expected warning message, got %+v", msg2)
	}
}

func TestEmitMessage_NoChannelDoesNotBlock(t *testing.T) {
	domain.DomainEventChan = nil
	// Calling EmitMessage without channel should be a no-op and not panic or block
	domain.EmitMessage(domain.MsgInfo, "no channel info")
}
