// Modified by VFS Platform contributors, 2026.
package modes

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/logdyhq/logdy-core/models"
	"github.com/stretchr/testify/assert"
)

func TestReadFilesLatestMergesSourcesByRecordTimestamp(t *testing.T) {
	dir := t.TempDir()
	newer := filepath.Join(dir, "newer.log")
	older := filepath.Join(dir, "older.log")
	assert.NoError(t, os.WriteFile(newer, []byte("2026-08-25 13:00:03,000 INFO newer: third\n2026-08-25 13:00:04,000 INFO newer: fourth\n"), 0o600))
	assert.NoError(t, os.WriteFile(older, []byte("2026-08-25 13:00:01,000 INFO older: first\n2026-08-25 13:00:02,000 INFO older: second\n"), 0o600))

	ch := make(chan models.Message, 3)
	ReadFilesLatest(ch, []string{newer, older}, 3)

	var got []string
	for range 3 {
		got = append(got, (<-ch).Content)
	}
	assert.Equal(t, []string{
		"2026-08-25 13:00:02,000 INFO older: second",
		"2026-08-25 13:00:03,000 INFO newer: third",
		"2026-08-25 13:00:04,000 INFO newer: fourth",
	}, got)
}

func TestLogTimestampReadsTunnelJSON(t *testing.T) {
	fallback := time.Unix(0, 0)
	got := logTimestamp(`{"time":"2026-08-25T13:58:24.0035901+02:00","level":"INFO"}`, fallback)
	assert.Equal(t, "2026-08-25T13:58:24.0035901+02:00", got.Format(time.RFC3339Nano))
}

func TestFollowFilesReceivesAppendedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bridge.stdout.log")
	if err := os.WriteFile(path, []byte("existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ch := make(chan models.Message, 1)
	FollowFiles(ch, []string{path})
	time.Sleep(200 * time.Millisecond)

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("online line\n"); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case message := <-ch:
		assert.Equal(t, "online line", message.Content)
	case <-time.After(3 * time.Second):
		t.Fatal("appended line was not delivered")
	}
}

func TestFollowFiles(t *testing.T) {

	ch := make(chan models.Message)
	ctx, cancel := context.WithCancel(context.Background())

	f, err := os.CreateTemp("", "sample")

	go func(ctx context.Context) {
		i := 0
		for {
			if ctx.Err() != nil {
				return
			}

			f.Write([]byte("foobar" + strconv.Itoa(i)))
			time.Sleep(1 * time.Millisecond)
			i++
		}
	}(ctx)

	assert.Equal(t, err, nil)

	go FollowFiles(ch, []string{f.Name()})

	received := 0
	for received < 20 {
		<-ch
		received++

	}
	cancel()

	assert.GreaterOrEqual(t, received, 20)

}
