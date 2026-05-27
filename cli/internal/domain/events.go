package domain

// MessageType represents the severity or category of a domain diagnostic message.
type MessageType string

const (
	MsgInfo    MessageType = "info"
	MsgWarning MessageType = "warning"
	MsgError   MessageType = "error"
)

// DomainMessage represents a diagnostic or progress message sent from
// the domain layer to the outer orchestration layer (commands).
type DomainMessage struct {
	Type    MessageType
	Message string
}

// DomainEventChan is a package-level Go channel that domain functions can
// write to without having direct console logging side effects. The commands
// layer can initialize and read from this channel as needed.
var DomainEventChan chan DomainMessage

// EmitMessage dispatches a message to DomainEventChan in a non-blocking fashion.
func EmitMessage(msgType MessageType, text string) {
	if DomainEventChan != nil {
		select {
		case DomainEventChan <- DomainMessage{Type: msgType, Message: text}:
		default:
			// Non-blocking write to avoid locking domain execution if the channel is unbuffered or full
		}
	}
}
