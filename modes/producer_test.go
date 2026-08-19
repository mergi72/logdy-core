// Modified by VFS Platform contributors, 2026.
package modes

import (
	"testing"
	"time"

	"github.com/logdyhq/logdy-core/models"
)

func TestProduceMessageIDsAreUniqueWithinSameTimestamp(t *testing.T) {
	const count = 1000
	ch := make(chan models.Message, count)
	ts := time.UnixMilli(1_700_000_000_000)

	for i := 0; i < count; i++ {
		ProduceMessageStringTimestamped(ch, "line", models.MessageTypeStdout, nil, ts)
	}

	ids := make(map[string]struct{}, count)
	for i := 0; i < count; i++ {
		message := <-ch
		if _, exists := ids[message.Id]; exists {
			t.Fatalf("duplicate message ID: %s", message.Id)
		}
		ids[message.Id] = struct{}{}
		if message.Ts != ts.UnixMilli() {
			t.Fatalf("unexpected timestamp: %d", message.Ts)
		}
	}
}
