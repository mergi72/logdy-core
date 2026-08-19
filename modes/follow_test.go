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
