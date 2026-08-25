// Modified by VFS Platform contributors, 2026.
package modes

import (
	"container/heap"
	"encoding/json"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/logdyhq/logdy-core/utils"

	"github.com/nxadm/tail"
	"github.com/sirupsen/logrus"

	"github.com/logdyhq/logdy-core/models"
)

func FollowFiles(ch chan models.Message, files []string) {

	for _, file := range files {

		_, err := os.Stat(file)
		if err != nil {
			utils.Logger.WithFields(logrus.Fields{
				"path":  file,
				"error": err.Error(),
			}).Error("Following file changes failed")
			continue
		}
		utils.Logger.WithFields(logrus.Fields{
			"path": file,
		}).Info("Following file changes")

		go func(file string) {
			t, err := tail.TailFile(
				file, tail.Config{Follow: true, ReOpen: true, Poll: true, Location: &tail.SeekInfo{Offset: 0, Whence: io.SeekEnd}})
			if err != nil {
				utils.Logger.WithFields(logrus.Fields{
					"path":  file,
					"error": err.Error(),
				}).Error("Following file changes failed")
			}

			for line := range t.Lines {
				ProduceFileMessageStringTimestamped(ch, line.Text, models.MessageTypeStdout, &models.MessageOrigin{File: file}, line.Time, line.SeekInfo.Offset)
			}

		}(file)
	}

}

func ReadFiles(ch chan models.Message, files []string) {
	for _, file := range files {
		_, err := os.Stat(file)
		if err != nil {
			utils.Logger.WithFields(logrus.Fields{
				"path":  file,
				"error": err.Error(),
			}).Error("Reading file failed")
			continue
		}

		r, size, bar := utils.OpenFileForReadingWithProgress(file)
		utils.Logger.WithFields(logrus.Fields{
			"path":       file,
			"size_bytes": size,
		}).Info("Reading file")

		utils.LineCounterWithChannel(r, func(line utils.Line, cancel func()) {
			ProduceFileMessageStringTimestamped(ch, string(line.Line), models.MessageTypeStdout, &models.MessageOrigin{File: file}, time.Now(), line.EndOffset)
		})
		if closer, ok := r.(io.Closer); ok {
			_ = closer.Close()
		}
		bar.Finish()
	}
}

type fileRecord struct {
	file      string
	line      string
	endOffset int64
	timestamp time.Time
	sequence  int64
}

type oldestRecords []fileRecord

func (r oldestRecords) Len() int { return len(r) }
func (r oldestRecords) Less(i, j int) bool {
	if r[i].timestamp.Equal(r[j].timestamp) {
		return r[i].sequence < r[j].sequence
	}
	return r[i].timestamp.Before(r[j].timestamp)
}
func (r oldestRecords) Swap(i, j int)   { r[i], r[j] = r[j], r[i] }
func (r *oldestRecords) Push(value any) { *r = append(*r, value.(fileRecord)) }
func (r *oldestRecords) Pop() any {
	old := *r
	last := old[len(old)-1]
	*r = old[:len(old)-1]
	return last
}

func ReadFilesLatest(ch chan models.Message, files []string, maxRecords int64) {
	records := &oldestRecords{}
	heap.Init(records)
	var sequence int64

	for _, file := range files {

		_, err := os.Stat(file)
		if err != nil {
			utils.Logger.WithFields(logrus.Fields{
				"path":  file,
				"error": err.Error(),
			}).Error("Reading file failed")
			continue
		}

		r, size, bar := utils.OpenFileForReadingWithProgress(file)
		utils.Logger.WithFields(logrus.Fields{
			"path":       file,
			"size_bytes": size,
		}).Info("Reading file")

		fallback := time.Now()
		utils.LineCounterWithChannel(r, func(line utils.Line, cancel func()) {
			sequence++
			text := string(line.Line)
			record := fileRecord{file: file, line: text, endOffset: line.EndOffset, timestamp: logTimestamp(text, fallback), sequence: sequence}
			heap.Push(records, record)
			if maxRecords > 0 && int64(records.Len()) > maxRecords {
				heap.Pop(records)
			}
		})
		if closer, ok := r.(io.Closer); ok {
			_ = closer.Close()
		}
		bar.Finish()
	}

	ordered := make([]fileRecord, records.Len())
	copy(ordered, *records)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].timestamp.Equal(ordered[j].timestamp) {
			return ordered[i].sequence < ordered[j].sequence
		}
		return ordered[i].timestamp.Before(ordered[j].timestamp)
	})
	for _, record := range ordered {
		ProduceFileMessageStringTimestamped(ch, record.line, models.MessageTypeStdout, &models.MessageOrigin{File: record.file}, record.timestamp, record.endOffset)
	}
}

func logTimestamp(line string, fallback time.Time) time.Time {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "{") {
		var payload struct {
			Time      string `json:"time"`
			Timestamp string `json:"timestamp"`
		}
		if json.Unmarshal([]byte(trimmed), &payload) == nil {
			value := payload.Time
			if value == "" {
				value = payload.Timestamp
			}
			if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
				return parsed
			}
		}
	}
	const layout = "2006-01-02 15:04:05,000"
	if len(trimmed) >= len(layout) {
		if parsed, err := time.ParseInLocation(layout, trimmed[:len(layout)], time.Local); err == nil {
			return parsed
		}
	}
	return fallback
}
