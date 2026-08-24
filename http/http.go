// Modified by VFS Platform contributors, 2026.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/logdyhq/logdy-core/utils"

	"github.com/sirupsen/logrus"
)

func getClientId(r *http.Request) (string, error) {
	kname := "logdy-client-id"
	cid := r.Header.Get(kname)

	if cid == "" {
		cid = r.URL.Query().Get(kname)
	}

	if cid == "" {
		return "", errors.New("missing client id")
	}

	return cid, nil
}

func getClientOrErr(r *http.Request, w http.ResponseWriter, clients *ClientsStruct) *Client {
	cid, err := getClientId(r)

	if err != nil {
		utils.Logger.Error("Missing client id")
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}

	cl, ok := clients.GetClient(cid)

	if !ok {
		utils.Logger.WithField("client_id", cid).Error("Missing client")
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}

	return cl
}

func httpError(err string, w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": err,
	})
}

func normalizeHttpPathPrefix(config *Config) {
	if len(config.HttpPathPrefix) > 0 && config.HttpPathPrefix[0] != byte('/') {
		config.HttpPathPrefix = "/" + config.HttpPathPrefix
	}

	if len(config.HttpPathPrefix) == 0 {
		config.HttpPathPrefix = "/"
	}

	if strings.LastIndex(config.HttpPathPrefix, "/") != len(config.HttpPathPrefix)-1 {
		config.HttpPathPrefix = config.HttpPathPrefix + "/"
	}
}

type hand interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
	Handle(pattern string, handler http.Handler)
}

func HandleHttp(config *Config, clients *ClientsStruct, serveMux hand) {

	assets, _ := Assets()

	if config.BulkWindowMs > 0 {
		BULK_WINDOW_MS.Store(config.BulkWindowMs)
	} else {
		BULK_WINDOW_MS.Store(100)
	}

	normalizeHttpPathPrefix(config)
	auth := newSessionAuth(config.UiPass, config.HttpPathPrefix)

	// Use the file system to serve static files
	fs := http.FileServer(http.FS(assets))

	v := reflect.ValueOf(serveMux)
	if serveMux == nil || v.IsNil() {
		utils.Logger.Debug("Using net/http")
		http.Handle(config.HttpPathPrefix, http.StripPrefix(config.HttpPathPrefix, fs))
		http.HandleFunc(config.HttpPathPrefix+"api/check-pass", handleCheckPass(auth))
		http.HandleFunc(config.HttpPathPrefix+"api/status", handleStatus(config, auth))
		http.HandleFunc(config.HttpPathPrefix+"api/client/set-status", auth.protect(handleClientStatus(clients)))
		http.HandleFunc(config.HttpPathPrefix+"api/client/load", auth.protect(handleClientLoad(clients)))
		http.HandleFunc(config.HttpPathPrefix+"api/client/peek-log", auth.protect(handleClientPeek(clients)))
		http.HandleFunc(config.HttpPathPrefix+"api/config/save", handleClientSettingsSave(auth))
		http.HandleFunc(config.HttpPathPrefix+"ws", handleWs(auth, clients, config.InitialMessageCount))

		http.HandleFunc(config.HttpPathPrefix+"api/log", apiKeyMiddleware(config.ApiKey, handleLog(Ch)))
	} else {
		utils.Logger.Debug("Using serveMux", serveMux)
		serveMux.Handle(config.HttpPathPrefix, http.StripPrefix(config.HttpPathPrefix, fs))
		serveMux.HandleFunc(config.HttpPathPrefix+"api/check-pass", handleCheckPass(auth))
		serveMux.HandleFunc(config.HttpPathPrefix+"api/status", handleStatus(config, auth))
		serveMux.HandleFunc(config.HttpPathPrefix+"api/client/set-status", auth.protect(handleClientStatus(clients)))
		serveMux.HandleFunc(config.HttpPathPrefix+"api/client/load", auth.protect(handleClientLoad(clients)))
		serveMux.HandleFunc(config.HttpPathPrefix+"api/client/peek-log", auth.protect(handleClientPeek(clients)))
		serveMux.HandleFunc(config.HttpPathPrefix+"api/config/save", handleClientSettingsSave(auth))
		serveMux.HandleFunc(config.HttpPathPrefix+"ws", handleWs(auth, clients, config.InitialMessageCount))

		serveMux.HandleFunc(config.HttpPathPrefix+"api/log", apiKeyMiddleware(config.ApiKey, handleLog(Ch)))
	}

}

func StartWebserver(config *Config) {
	utils.Logger.Debug("Starting webserver")
	utils.Logger.WithFields(logrus.Fields{
		"port": config.ServerPort,
	}).Info("WebUI started, visit http://" + config.ServerIp + ":" + config.ServerPort + config.HttpPathPrefix)

	server := &http.Server{
		Addr:              config.ServerIp + ":" + config.ServerPort,
		Handler:           nil,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	err := server.ListenAndServe()

	if err != nil {
		panic(err)
	}
}

type Config struct {
	AnalyticsDisabled bool
	UiPass            string
	ConfigFilePath    string
	BulkWindowMs      int64
	HttpPathPrefix    string
	ApiKey            string

	ServerPort string
	ServerIp   string

	AppendToFile              string
	AppendToFileRotateMaxSize string
	AppendToFileRaw           bool
	MaxMessageCount           int64
	InitialMessageCount       int64

	LogLevel       utils.LOG_LEVEL
	LogInterceptor utils.LogInterceptor
}
