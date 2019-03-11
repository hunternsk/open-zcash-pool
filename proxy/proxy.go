package proxy

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jkkgbe/open-zcash-pool/rpc"
	"github.com/jkkgbe/open-zcash-pool/storage"
	"github.com/jkkgbe/open-zcash-pool/util"
)

type ProxyServer struct {
	config             *Config
	work               atomic.Value
	upstream           int32
	upstreams          []*rpc.RPCClient
	backend            *storage.RedisClient
	diff               string
	hashrateExpiration time.Duration
	failsCount         int64

	extraNonceCounter uint32

	// Stratum
	sessionsMu sync.RWMutex
	sessions   map[*Session]struct{}
	timeout    time.Duration
}

type Session struct {
	ip  string
	enc *json.Encoder

	// Stratum
	sync.Mutex
	conn        *net.TCPConn
	login       string
	extraNonce1 string
}

func NewProxy(cfg *Config, backend *storage.RedisClient) *ProxyServer {
	if len(cfg.Name) == 0 {
		log.Fatal("You must set instance name")
	}

	proxy := &ProxyServer{
		config:             cfg,
		upstreams:          make([]*rpc.RPCClient, len(cfg.Upstream)),
		backend:            backend,
		diff:               util.GetTargetHex(cfg.Proxy.Difficulty),
		hashrateExpiration: util.MustParseDuration(cfg.Proxy.HashrateExpiration),

		extraNonceCounter: util.CreateExtraNonceCounter(cfg.InstanceId),
	}

	for i, upstream := range cfg.Upstream {
		proxy.upstreams[i] = rpc.NewRPCClient(upstream.Name, upstream.Url, upstream.Timeout)
		log.Printf("Upstream: %s => %s", upstream.Name, upstream.Url)
	}
	log.Printf("Default upstream: %s => %s", proxy.rpc().Name, proxy.rpc().Url)

	if cfg.Proxy.Stratum.Enabled {
		proxy.sessions = make(map[*Session]struct{})
		go proxy.ListenTCP()
	}

	proxy.fetchWork()

	refreshInterval := util.MustParseDuration(cfg.Proxy.BlockRefreshInterval)
	refreshTimer := time.NewTimer(refreshInterval)
	log.Printf("Set block refresh every %v", refreshInterval)

	checkInterval := util.MustParseDuration(cfg.UpstreamCheckInterval)
	checkTimer := time.NewTimer(refreshInterval)

	stateUpdateInterval := util.MustParseDuration(cfg.Proxy.StateUpdateInterval)
	stateUpdateTimer := time.NewTimer(stateUpdateInterval)

	go func() {
		for {
			select {
			case <-refreshTimer.C:
				proxy.fetchWork()
				refreshTimer.Reset(refreshInterval)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-checkTimer.C:
				proxy.checkUpstreams()
				checkTimer.Reset(checkInterval)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-stateUpdateTimer.C:
				currentWork := proxy.currentWork()
				if currentWork != nil {
					err := backend.WriteNodeState(cfg.Name, currentWork.Height, currentWork.Difficulty)
					if err != nil {
						log.Printf("Failed to write node state to backend: %v", err)
						proxy.markSick()
					} else {
						proxy.markOk()
					}
				}
				stateUpdateTimer.Reset(stateUpdateInterval)
			}
		}
	}()

	return proxy
}

func (proxyServer *ProxyServer) nextExtraNonce1() string {
	extraNonce1 := make([]byte, 4)
	binary.BigEndian.PutUint32(extraNonce1, proxyServer.extraNonceCounter)
	proxyServer.extraNonceCounter += 1
	return hex.EncodeToString(extraNonce1)
}

func (proxyServer *ProxyServer) rpc() *rpc.RPCClient {
	i := atomic.LoadInt32(&proxyServer.upstream)
	return proxyServer.upstreams[i]
}

func (proxyServer *ProxyServer) checkUpstreams() {
	candidate := int32(0)
	backup := false

	for i, upstream := range proxyServer.upstreams {
		if upstream.Check() && !backup {
			candidate = int32(i)
			backup = true
		}
	}

	if proxyServer.upstream != candidate {
		log.Printf("Switching to %v upstream", proxyServer.upstreams[candidate].Name)
		atomic.StoreInt32(&proxyServer.upstream, candidate)
	}
}

func (proxyServer *ProxyServer) currentWork() *Work {
	work := proxyServer.work.Load()
	if work != nil {
		return work.(*Work)
	} else {
		return nil
	}
}

func (proxyServer *ProxyServer) markSick() {
	atomic.AddInt64(&proxyServer.failsCount, 1)
}

func (proxyServer *ProxyServer) isSick() bool {
	failsCount := atomic.LoadInt64(&proxyServer.failsCount)
	if proxyServer.config.Proxy.HealthCheck && failsCount >= proxyServer.config.Proxy.MaxFails {
		return true
	}
	return false
}

func (proxyServer *ProxyServer) markOk() {
	atomic.StoreInt64(&proxyServer.failsCount, 0)
}
