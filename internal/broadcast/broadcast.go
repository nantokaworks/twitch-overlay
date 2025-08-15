package broadcast

import (
	"github.com/nantokaworks/twitch-overlay/internal/faxmanager"
)

// FaxBroadcaster is an interface for broadcasting fax events
type FaxBroadcaster interface {
	BroadcastFax(fax *faxmanager.Fax)
}

// MessageBroadcaster is an interface for broadcasting generic messages
type MessageBroadcaster interface {
	BroadcastMessage(message interface{})
}

// Broadcaster combines both interfaces
type Broadcaster interface {
	FaxBroadcaster
	MessageBroadcaster
}

// Global broadcaster instance
var globalBroadcaster Broadcaster

// SetBroadcaster sets the global broadcaster instance
func SetBroadcaster(b Broadcaster) {
	globalBroadcaster = b
}

// BroadcastFax broadcasts a fax event using the global broadcaster
func BroadcastFax(fax *faxmanager.Fax) {
	if globalBroadcaster != nil {
		globalBroadcaster.BroadcastFax(fax)
	}
}

// Send broadcasts a generic message using the global broadcaster
func Send(message interface{}) {
	if globalBroadcaster != nil {
		globalBroadcaster.BroadcastMessage(message)
	}
}