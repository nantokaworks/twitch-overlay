package broadcast

import (
	"github.com/nantokaworks/twitch-overlay/internal/faxmanager"
)

// FaxBroadcaster is an interface for broadcasting fax events
type FaxBroadcaster interface {
	BroadcastFax(fax *faxmanager.Fax)
}

// Global broadcaster instance
var globalBroadcaster FaxBroadcaster

// SetBroadcaster sets the global broadcaster instance
func SetBroadcaster(b FaxBroadcaster) {
	globalBroadcaster = b
}

// BroadcastFax broadcasts a fax event using the global broadcaster
func BroadcastFax(fax *faxmanager.Fax) {
	if globalBroadcaster != nil {
		globalBroadcaster.BroadcastFax(fax)
	}
}