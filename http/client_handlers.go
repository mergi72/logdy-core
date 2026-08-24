// Modified by VFS Platform contributors, 2026.
package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/logdyhq/logdy-core/models"
	"github.com/logdyhq/logdy-core/utils"
	"github.com/sirupsen/logrus"
)

const LOGDY_CONFIG_ENV_FILE = "logdy.config.json"
const MAX_CONFIG_LAYOUT_BYTES = 1 << 20

func handleCheckPass(auth *sessionAuth) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		utils.Logger.Debug("/api/check-pass")
		if auth.required() && r.Method == http.MethodPost {
			utils.Logger.WithFields(logrus.Fields{
				"ip": r.RemoteAddr,
				"ua": r.Header.Get("user-agent"),
			}).Debug("Client login attempt")
		}
		auth.check(w, r)
	}
}

func handleStatus(config *Config, auth *sessionAuth) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		utils.Logger.Debug("/api/status")

		configStr := ""
		authenticated := auth.authenticated(r)
		if authenticated && config.ConfigFilePath != "" {
			utils.Logger.WithFields(logrus.Fields{
				"file": config.ConfigFilePath,
			}).Debug("Reading config file")
			configStr = utils.LoadFile(config.ConfigFilePath)
		} else if authenticated && utils.FileExists(LOGDY_CONFIG_ENV_FILE) {
			utils.Logger.WithFields(logrus.Fields{
				"file": LOGDY_CONFIG_ENV_FILE,
			}).Info("Loading local env file")
			configStr = utils.LoadFile(LOGDY_CONFIG_ENV_FILE)
		}

		newVersion := models.LogdyVersionUpdateResponse{}
		if authenticated {
			newVersion, _ = utils.IsNewVersionAvailable()
		}

		initMsg, _ := json.Marshal(models.InitMessage{
			BaseMessage: models.BaseMessage{
				MessageType: models.MessageTypeInit,
			},
			AnalyticsEnabled: authenticated && !config.AnalyticsDisabled,
			AuthRequired:     config.UiPass != "",
			ConfigStr:        configStr,
			ApiPrefix:        config.HttpPathPrefix,
			UpdateVersion:    newVersion,
		})

		w.Header().Add("content-type", "application/json")
		w.Write(initMsg)
	}
}

func handleWs(auth *sessionAuth, clients *ClientsStruct) func(w http.ResponseWriter, r *http.Request) {

	wsUpgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     sameHostOrigin,
	}
	return func(w http.ResponseWriter, r *http.Request) {

		if !auth.authenticated(r) {
			utils.Logger.WithFields(logrus.Fields{
				"ip": r.RemoteAddr,
				"ua": r.Header.Get("user-agent"),
			}).Info("Client denied")
			w.WriteHeader(http.StatusForbidden)
			return
		}

		// Upgrade the HTTP connection to a WebSocket connection.
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Println(err)
			return
		}

		utils.Logger.Info("New Web UI client connected")

		ch := clients.Join(int(clients.Stats().MaxCount), r.URL.Query().Get("should_follow") == "true")
		clientId := ch.id

		bts, err := json.Marshal(models.ClientJoined{
			BaseMessage: models.BaseMessage{
				MessageType: models.MessageTypeClientJoined,
			},
			ClientId: ch.id,
		})

		if err != nil {
			fmt.Println(err)
			return
		}

		err = conn.WriteMessage(1, bts)

		if err != nil {
			fmt.Println(err)
			return
		}

		go func(clienId string) {
			for {
				time.Sleep(1 * time.Second)
				_, _, err := conn.ReadMessage()
				if err != nil {
					utils.Logger.Debug(err)
					utils.Logger.WithField("client_id", clientId).Info("Closed client")
					clients.Close(clientId)
					return
				}
			}
		}(clientId)

		mtx := sync.Mutex{}

		go func(clientId string) {
			for {
				time.Sleep(1 * time.Second)
				if ch.isCursorStopped() {
					statusBytes, marshalErr := json.Marshal(models.ClientMsgStatus{
						BaseMessage: models.BaseMessage{
							MessageType: models.MessageTypeClientMsgStatus,
						},
						Client: clients.ClientStats(ch.id),
						Stats:  clients.Stats(),
					})
					if marshalErr != nil {
						fmt.Println("Error while serializing message", marshalErr)
						continue
					}

					mtx.Lock()
					writeErr := conn.WriteMessage(1, statusBytes)
					mtx.Unlock()

					if writeErr != nil {
						utils.Logger.Error("Err", writeErr)
						clients.Close(clientId)
						utils.Logger.WithField("client_id", clientId).Info("Closed client")
						break
					}

				}
			}
		}(clientId)

		for {
			msgs := <-ch.ch
			bulk := models.MessageBulk{
				BaseMessage: models.BaseMessage{
					MessageType: models.MessageTypeLogBulk,
				},
				Messages: msgs,
				Status:   clients.Stats(),
			}

			utils.Logger.WithField("count", len(msgs)).Debug("Received messages")

			if utils.Logger.Level <= logrus.DebugLevel {
				for _, msg := range msgs {
					mbts, _ := json.Marshal(msg)
					utils.Logger.WithFields(logrus.Fields{
						"msg":      utils.Trunc(string(mbts), 45),
						"clientId": clientId,
					}).Debug("Sending message through WebSocket")
				}
			}

			bulkBytes, err := json.Marshal(bulk)
			if err != nil {
				fmt.Println("Error while serializing message", err)
				continue
			}

			mtx.Lock()
			err = conn.WriteMessage(1, bulkBytes)

			if err != nil {
				utils.Logger.Error("Err", err)
				clients.Close(clientId)
				utils.Logger.WithField("client_id", clientId).Info("Closed client")
				break
			}

			statusBytes, err := json.Marshal(models.ClientMsgStatus{
				BaseMessage: models.BaseMessage{
					MessageType: models.MessageTypeClientMsgStatus,
				},
				Client: clients.ClientStats(ch.id),
				Stats:  clients.Stats(),
			})

			if err != nil {
				fmt.Println("Error while serializing message", err)
				continue
			}

			err = conn.WriteMessage(1, statusBytes)
			mtx.Unlock()

			if err != nil {
				utils.Logger.Error("Err", err)
				clients.Close(clientId)
				utils.Logger.WithField("client_id", clientId).Info("Closed client")
				break
			}
		}

	}
}

func sameHostOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}

	return parsed.Host == r.Host
}

func handleClientSettingsSave(auth ...*sessionAuth) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameHostOrigin(r) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		if len(auth) > 0 && !auth[0].authenticated(r) {
			http.Error(w, "authentication required", http.StatusForbidden)
			return
		}

		type Req struct {
			Layout string `json:"layout"`
		}

		var p Req
		r.Body = http.MaxBytesReader(w, r.Body, MAX_CONFIG_LAYOUT_BYTES)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		err := decoder.Decode(&p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err = os.WriteFile(LOGDY_CONFIG_ENV_FILE, []byte(p.Layout), 0644)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		path, err := os.Getwd()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"location": path + string(os.PathSeparator) + LOGDY_CONFIG_ENV_FILE,
		})
	}
}

func handleClientStatus(clients *ClientsStruct) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cl := getClientOrErr(r, w, clients)
		if cl == nil {
			return
		}

		status := r.URL.Query().Get("status")

		switch status {
		case string(CURSOR_FOLLOWING):
			clients.ResumeFollowing(cl.id, r.URL.Query().Has("from_cursor"))
		case string(CURSOR_STOPPED):
			clients.PauseFollowing(cl.id)
		default:
			utils.Logger.Error("Unrecognized status")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func handleClientLoad(clients *ClientsStruct) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cl := getClientOrErr(r, w, clients)
		if cl == nil {
			return
		}

		start := r.URL.Query().Get("start")
		count := r.URL.Query().Get("count")

		startInt, err := strconv.Atoi(start)
		if err != nil {
			utils.Logger.Error("Invalid start")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		countInt, err := strconv.Atoi(count)
		if err != nil {
			utils.Logger.Error("Invalid count")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		clients.Load(cl.id, startInt, countInt, true)

		w.WriteHeader(http.StatusOK)
	}
}

func handleClientPeek(clients *ClientsStruct) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cl := getClientOrErr(r, w, clients)
		if cl == nil {
			return
		}

		type Req struct {
			Idxs []int `json:"idxs"`
		}

		var p Req

		// Try to decode the request body into the struct. If there is an error,
		// respond to the client with the error message and a 400 status code.
		err := json.NewDecoder(r.Body).Decode(&p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		msgs := clients.PeekLog(p.Idxs)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(msgs)

	}
}
