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

func TestProduceFileMessageIDIsStableForSamePhysicalLine(t *testing.T) {
	ch := make(chan models.Message, 3)
	origin := &models.MessageOrigin{File: `C:\logs\bridge.log`}

	ProduceFileMessageStringTimestamped(ch, "same line", models.MessageTypeStdout, origin, time.Unix(1, 0), 42)
	ProduceFileMessageStringTimestamped(ch, "same line", models.MessageTypeStdout, origin, time.Unix(2, 0), 42)
	ProduceFileMessageStringTimestamped(ch, "changed line", models.MessageTypeStdout, origin, time.Unix(2, 0), 42)

	first := <-ch
	second := <-ch
	changed := <-ch
	if first.Id != second.Id {
		t.Fatalf("same physical line IDs differ: %q != %q", first.Id, second.Id)
	}
	if first.Id == changed.Id {
		t.Fatalf("changed physical line reused ID: %q", first.Id)
	}
}

func TestProduceMessageDecodesWindows1250(t *testing.T) {
	ch := make(chan models.Message, 1)
	line := string([]byte{0x4e, 0x65, 0x6d, 0x6f, 0x68, 0x6c, 0x6f, 0x20, 0x62, 0xfd, 0x74, 0x20, 0x76, 0x79, 0x74, 0x76, 0x6f, 0xf8, 0x65, 0x6e, 0x6f, 0x20, 0x70, 0xf8, 0x69, 0x70, 0x6f, 0x6a, 0x65, 0x6e, 0xed})

	ProduceMessageStringTimestamped(ch, line, models.MessageTypeStdout, nil, time.Now())

	if got := (<-ch).Content; got != "Nemohlo být vytvořeno připojení" {
		t.Fatalf("decoded content = %q", got)
	}
}
