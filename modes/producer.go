// Modified by VFS Platform contributors, 2026.
package modes

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/logdyhq/logdy-core/models"
	"github.com/logdyhq/logdy-core/utils"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fastjson"
	"golang.org/x/text/encoding/charmap"
)

var FallthroughGlobal = false
var DisableANSICodeStripping = false
var messageSequence atomic.Uint64

func ProduceMessageStringTimestamped(ch chan models.Message, line string, mt models.LogType, mo *models.MessageOrigin, ts time.Time) {
	produceMessageStringTimestamped(ch, line, mt, mo, ts, "")
}

func ProduceFileMessageStringTimestamped(ch chan models.Message, line string, mt models.LogType, mo *models.MessageOrigin, ts time.Time, endOffset int64) {
	identity := fmt.Sprintf("%s\x00%d\x00%s", strings.ToLower(mo.File), endOffset, line)
	stableID := fmt.Sprintf("file-%x", sha256.Sum256([]byte(identity)))
	produceMessageStringTimestamped(ch, line, mt, mo, ts, stableID)
}

func produceMessageStringTimestamped(ch chan models.Message, line string, mt models.LogType, mo *models.MessageOrigin, ts time.Time, messageID string) {
	if !utf8.ValidString(line) {
		if decoded, err := charmap.Windows1250.NewDecoder().String(line); err == nil {
			line = decoded
		} else {
			line = strings.ToValidUTF8(line, "�")
		}
	}

	if !DisableANSICodeStripping {
		line = utils.StripAnsi(line)
	}

	validJson := fastjson.Validate(line)
	var cs json.RawMessage
	if validJson == nil {
		cs = json.RawMessage(line)
	}

	fields := logrus.Fields{
		"line": utils.Trunc(line, 45),
	}
	if mo != nil {
		if mo.Port != "" {
			fields["origin_port"] = mo.Port
		}
		if mo.File != "" {
			fields["origin_file"] = mo.File
		}
	}

	utils.Logger.WithFields(fields).Debug("Producing message")

	if FallthroughGlobal {
		if mt == models.MessageTypeStdout {
			fmt.Fprintln(os.Stdout, line)
		}
		if mt == models.MessageTypeStderr {
			fmt.Fprintln(os.Stderr, line)
		}
	}

	if messageID == "" {
		messageID = strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + strconv.FormatUint(messageSequence.Add(1), 10)
	}

	ch <- models.Message{
		Id:          messageID,
		Mtype:       mt,
		Content:     line,
		JsonContent: cs,
		IsJson:      validJson == nil,
		BaseMessage: models.BaseMessage{MessageType: "log"},
		Origin:      mo,
		Ts:          ts.UnixMilli(),
	}
}

func ProduceMessageString(ch chan models.Message, line string, mt models.LogType, mo *models.MessageOrigin) {
	ProduceMessageStringTimestamped(ch, line, mt, mo, time.Now())
}
