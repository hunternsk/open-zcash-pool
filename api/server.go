package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"

	"github.com/jkkgbe/open-zcash-pool/storage"
	"github.com/jkkgbe/open-zcash-pool/util"
)

type ApiConfig struct {
	Enabled              bool   `json:"enabled"`
	Listen               string `json:"listen"`
	StatsCollectInterval string `json:"statsCollectInterval"`
	HashrateWindow       string `json:"hashrateWindow"`
	HashrateLargeWindow  string `json:"hashrateLargeWindow"`
	LuckWindow           []int  `json:"luckWindow"`
	Blocks               int64  `json:"blocks"`
	PurgeOnly            bool   `json:"purgeOnly"`
	PurgeInterval        string `json:"purgeInterval"`
}

type ApiServer struct {
	config              *ApiConfig
	backend             *storage.RedisClient
	hashrateWindow      time.Duration
	hashrateLargeWindow time.Duration
	stats               atomic.Value
	miners              map[string]*Entry
	minersMu            sync.RWMutex
	statsIntv           time.Duration
}

type Entry struct {
	stats     map[string]interface{}
	updatedAt int64
}

func NewApiServer(cfg *ApiConfig, backend *storage.RedisClient) *ApiServer {
	hashrateWindow := util.MustParseDuration(cfg.HashrateWindow)
	hashrateLargeWindow := util.MustParseDuration(cfg.HashrateLargeWindow)
	return &ApiServer{
		config:              cfg,
		backend:             backend,
		hashrateWindow:      hashrateWindow,
		hashrateLargeWindow: hashrateLargeWindow,
		miners:              make(map[string]*Entry),
	}
}

func (apiServer *ApiServer) Start() {
	if apiServer.config.PurgeOnly {
		log.Printf("Starting API in purge-only mode")
	} else {
		log.Printf("Starting API on %v", apiServer.config.Listen)
	}

	apiServer.statsIntv = util.MustParseDuration(apiServer.config.StatsCollectInterval)
	statsTimer := time.NewTimer(apiServer.statsIntv)
	log.Printf("Set stats collect interval to %v", apiServer.statsIntv)

	purgeIntv := util.MustParseDuration(apiServer.config.PurgeInterval)
	purgeTimer := time.NewTimer(purgeIntv)
	log.Printf("Set purge interval to %v", purgeIntv)

	sort.Ints(apiServer.config.LuckWindow)

	if apiServer.config.PurgeOnly {
		apiServer.purgeStale()
	} else {
		apiServer.purgeStale()
		apiServer.collectStats()
	}

	go func() {
		for {
			select {
			case <-statsTimer.C:
				if !apiServer.config.PurgeOnly {
					apiServer.collectStats()
				}
				statsTimer.Reset(apiServer.statsIntv)
			case <-purgeTimer.C:
				apiServer.purgeStale()
				purgeTimer.Reset(purgeIntv)
			}
		}
	}()

	if !apiServer.config.PurgeOnly {
		apiServer.listen()
	}
}

func (apiServer *ApiServer) listen() {
	router := mux.NewRouter()
	router.HandleFunc("/api/stats", apiServer.StatsIndex)
	router.HandleFunc("/api/miners", apiServer.MinersIndex)
	router.HandleFunc("/api/blocks", apiServer.BlocksIndex)
	router.HandleFunc("/api/accounts/{login:t[0-9a-zA-Z]{34}}", apiServer.AccountIndex)
	router.NotFoundHandler = http.HandlerFunc(notFound)
	err := http.ListenAndServe(apiServer.config.Listen, router)
	if err != nil {
		log.Fatalf("Failed to start API: %v", err)
	}
}

func notFound(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=UTF-8")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.WriteHeader(http.StatusNotFound)
}

func (apiServer *ApiServer) purgeStale() {
	start := time.Now()
	total, err := apiServer.backend.FlushStaleStats(apiServer.hashrateWindow, apiServer.hashrateLargeWindow)
	if err != nil {
		log.Println("Failed to purge stale data from backend:", err)
	} else {
		log.Printf("Purged stale stats from backend, %v shares affected, elapsed time %v", total, time.Since(start))
	}
}

func (apiServer *ApiServer) collectStats() {
	start := time.Now()
	stats, err := apiServer.backend.CollectStats(apiServer.hashrateWindow, apiServer.config.Blocks)
	if err != nil {
		log.Printf("Failed to fetch stats from backend: %v", err)
		return
	}
	if len(apiServer.config.LuckWindow) > 0 {
		stats["luck"], err = apiServer.backend.CollectLuckStats(apiServer.config.LuckWindow)
		if err != nil {
			log.Printf("Failed to fetch luck stats from backend: %v", err)
			return
		}
	}
	apiServer.stats.Store(stats)
	log.Printf("Stats collection finished %s", time.Since(start))
}

func (apiServer *ApiServer) StatsIndex(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=UTF-8")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.WriteHeader(http.StatusOK)

	reply := make(map[string]interface{})
	nodes, err := apiServer.backend.GetNodeStates()
	if err != nil {
		log.Printf("Failed to get nodes stats from backend: %v", err)
	}
	reply["nodes"] = nodes

	stats := apiServer.getStats()
	if stats != nil {
		reply["now"] = util.MakeTimestamp()
		reply["stats"] = stats["stats"]
		reply["hashrate"] = stats["hashrate"]
		reply["minersTotal"] = stats["minersTotal"]
		reply["maturedTotal"] = stats["maturedTotal"]
		reply["immatureTotal"] = stats["immatureTotal"]
		reply["candidatesTotal"] = stats["candidatesTotal"]
	}

	err = json.NewEncoder(writer).Encode(reply)
	if err != nil {
		log.Println("Error serializing API response: ", err)
	}
}

func (apiServer *ApiServer) MinersIndex(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=UTF-8")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.WriteHeader(http.StatusOK)

	reply := make(map[string]interface{})
	stats := apiServer.getStats()
	if stats != nil {
		reply["now"] = util.MakeTimestamp()
		reply["miners"] = stats["miners"]
		reply["hashrate"] = stats["hashrate"]
		reply["minersTotal"] = stats["minersTotal"]
	}

	err := json.NewEncoder(writer).Encode(reply)
	if err != nil {
		log.Println("Error serializing API response: ", err)
	}
}

func (apiServer *ApiServer) BlocksIndex(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=UTF-8")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.WriteHeader(http.StatusOK)

	reply := make(map[string]interface{})
	stats := apiServer.getStats()
	if stats != nil {
		reply["matured"] = stats["matured"]
		reply["maturedTotal"] = stats["maturedTotal"]
		reply["immature"] = stats["immature"]
		reply["immatureTotal"] = stats["immatureTotal"]
		reply["candidates"] = stats["candidates"]
		reply["candidatesTotal"] = stats["candidatesTotal"]
		reply["luck"] = stats["luck"]
	}

	err := json.NewEncoder(writer).Encode(reply)
	if err != nil {
		log.Println("Error serializing API response: ", err)
	}
}

func (apiServer *ApiServer) AccountIndex(writer http.ResponseWriter, r *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=UTF-8")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Cache-Control", "no-cache")

	login := mux.Vars(r)["login"]
	apiServer.minersMu.Lock()
	defer apiServer.minersMu.Unlock()

	reply, ok := apiServer.miners[login]
	now := util.MakeTimestamp()
	cacheIntv := int64(apiServer.statsIntv / time.Millisecond)
	// Refresh stats if stale
	if !ok || reply.updatedAt < now-cacheIntv {
		exist, err := apiServer.backend.IsMinerExists(login)
		if !exist {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			log.Printf("Failed to fetch stats from backend: %v", err)
			return
		}

		stats, err := apiServer.backend.GetMinerStats(login)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			log.Printf("Failed to fetch stats from backend: %v", err)
			return
		}
		workers, err := apiServer.backend.CollectWorkersStats(apiServer.hashrateWindow, apiServer.hashrateLargeWindow, login)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			log.Printf("Failed to fetch stats from backend: %v", err)
			return
		}
		for key, value := range workers {
			stats[key] = value
		}

		reply = &Entry{stats: stats, updatedAt: now}
		apiServer.miners[login] = reply
	}

	writer.WriteHeader(http.StatusOK)
	err := json.NewEncoder(writer).Encode(reply.stats)
	if err != nil {
		log.Println("Error serializing API response: ", err)
	}
}

func (apiServer *ApiServer) getStats() map[string]interface{} {
	stats := apiServer.stats.Load()
	if stats != nil {
		return stats.(map[string]interface{})
	}
	return nil
}
