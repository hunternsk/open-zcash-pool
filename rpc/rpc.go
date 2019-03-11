package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"

	"github.com/jkkgbe/open-zcash-pool/util"
)

type Tx struct {
	Hash string `json:"hash"`
}
type GetMiningInfo struct {
	Blocks       int64   `json:"blocks"`
	Difficulty   float64 `json:"difficulty"`
	NetworkSolPS int64   `json:"networksolps"`
	Testnet      bool    `json:"testnet"`
	Chain        string  `json:"chain"`
}

type GetBlockReply struct {
	Hash          string  `json:"hash"`
	Confirmations int64   `json:"confirmations"`
	Height        int64   `json:"height"`
	Transactions  []Tx    `json:"transactions"`
	Nonce         string  `json:"nonce"`
	Difficulty    float64 `json:"difficulty"`
}

type RPCClient struct {
	sync.RWMutex
	Url         string
	Name        string
	sick        bool
	sickRate    int
	successRate int
	client      *http.Client
}

type JSONRpcResp struct {
	Id     *json.RawMessage       `json:"id"`
	Result *json.RawMessage       `json:"result"`
	Error  map[string]interface{} `json:"error"`
}

func NewRPCClient(name, url, timeout string) *RPCClient {
	rpcClient := &RPCClient{Name: name, Url: url}
	timeoutIntv := util.MustParseDuration(timeout)
	rpcClient.client = &http.Client{
		Timeout: timeoutIntv,
	}
	return rpcClient
}

func (r *RPCClient) GetBlockByHeight(height int64) (*GetBlockReply, error) {
	rpcResp, err := r.doPost(r.Url, "getblock", []string{strconv.FormatInt(height, 10)})
	if err != nil {
		return nil, err
	}

	var reply *GetBlockReply
	if rpcResp.Result != nil {
		err = json.Unmarshal(*rpcResp.Result, &reply)
	}

	return reply, err
}

func (r *RPCClient) GetMiningInfo() (*GetMiningInfo, error) {
	rpcResp, err := r.doPost(r.Url, "getmininginfo", []string{})
	if err != nil {
		return nil, err
	}
	if rpcResp.Result != nil {
		var reply *GetMiningInfo
		err = json.Unmarshal(*rpcResp.Result, &reply)
		return reply, err
	}
	return nil, nil
}

func (r *RPCClient) GetBlockTemplate(reply interface{}) error {
	rpcResp, err := r.doPost(r.Url, "getblocktemplate", []string{})
	if err != nil {
		return err
	}
	err = json.Unmarshal(*rpcResp.Result, reply)
	return err
}

func (r *RPCClient) SubmitBlock(header string) (interface{}, error) {
	rpcResp, err := r.doPost(r.Url, "submitblock", []string{header})

	if err != nil {
		return false, err
	}

	var reply interface{}
	if rpcResp.Result != nil {
		err = json.Unmarshal(*rpcResp.Result, reply)
	}

	return reply, err
}

func (r *RPCClient) doPost(url string, method string, params interface{}) (*JSONRpcResp, error) {
	jsonReq := map[string]interface{}{"jsonrpc": "2.0", "method": method, "params": params, "id": 0}

	data, _ := json.Marshal(jsonReq)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))

	if err != nil {
		r.markSick()
		return nil, err
	}

	req.Header.Set("Content-Length", (string)(len(data)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)

	if err != nil {
		r.markSick()
		return nil, err
	}
	defer resp.Body.Close()

	var rpcResp *JSONRpcResp

	err = json.NewDecoder(resp.Body).Decode(&rpcResp)

	if err != nil {
		r.markSick()
		return nil, err
	}
	if rpcResp.Error != nil {
		r.markSick()
		return nil, errors.New(rpcResp.Error["message"].(string))
	}

	return rpcResp, err
}

func (r *RPCClient) Check() bool {
	r.markAlive()
	return !r.Sick()
}

func (r *RPCClient) Sick() bool {
	r.RLock()
	defer r.RUnlock()
	return r.sick
}

func (r *RPCClient) markSick() {
	r.Lock()
	r.sickRate++
	r.successRate = 0
	if r.sickRate >= 5 {
		r.sick = true
	}
	r.Unlock()
}

func (r *RPCClient) markAlive() {
	r.Lock()
	r.successRate++
	if r.successRate >= 5 {
		r.sick = false
		r.sickRate = 0
		r.successRate = 0
	}
	r.Unlock()
}
