package proxy

import (
	"log"
	"math/big"
	"sync"

	"github.com/jkkgbe/open-zcash-pool/merkleTree"
	"github.com/jkkgbe/open-zcash-pool/transaction"
	"github.com/jkkgbe/open-zcash-pool/util"
)

type Transaction struct {
	Data string `json:"data"`
	Hash string `json:"hash"`
	Fee  int64  `json:"fee"`
}

type CoinbaseTxn struct {
	Data           string `json:"data"`
	Hash           string `json:"hash"`
	FoundersReward int64  `json:"foundersreward"`
}

type BlockTemplate struct {
	sync.RWMutex
	Version              uint32        `json:"version"`
	PrevBlockHash        string        `json:"previousblockhash"`
	FinalSaplingRootHash string        `json:"finalsaplingroothash"`
	Transactions         []Transaction `json:"transactions"`
	CoinbaseTxn          CoinbaseTxn   `json:"coinbasetxn"`
	LongpollId           string        `json:"longpollid"`
	Target               string        `json:"target"`
	MinTime              int           `json:"mintime"`
	NonceRange           string        `json:"noncerange"`
	SigOpLimit           int           `json:"sigoplimit"`
	SizeLimit            int           `json:"sizelimit"`
	CurTime              uint32        `json:"curtime"`
	Bits                 string        `json:"bits"`
	Height               int64         `json:"height"`
}

type Work struct {
	JobId                string
	Version              string
	PrevHashReversed     string
	MerkleRootReversed   string
	FinalSaplingRootHash string
	Time                 string
	Bits                 string
	Target               string
	Height               int64
	Difficulty           *big.Int
	CleanJobs            bool
	Template             *BlockTemplate
	GeneratedCoinbase    []byte
	FeeReward            int64
}

func (proxyServer *ProxyServer) fetchWork() {
	rpc := proxyServer.rpc()
	currentWork := proxyServer.currentWork()

	var blockTemplate BlockTemplate
	err := rpc.GetBlockTemplate(&blockTemplate)
	if err != nil {
		log.Printf("Error while refreshing block template on %s: %s", rpc.Name, err)
		return
	}

	// No need to update, we already have a fresh job
	if currentWork != nil && util.ReverseHex(currentWork.PrevHashReversed) == blockTemplate.PrevBlockHash {
		return
	}

	var feeReward int64 = 0
	for _, transaction := range blockTemplate.Transactions {
		feeReward += transaction.Fee
	}

	coinbaseTxn, coinbaseHash := transaction.BuildCoinbaseTxn(
		blockTemplate.Height,
		proxyServer.config.PoolAddress,
		blockTemplate.CoinbaseTxn.FoundersReward,
		feeReward,
	)

	txHashes := make([][32]byte, len(blockTemplate.Transactions)+1)
	copy(txHashes[0][:], coinbaseHash[:])

	for i, transaction := range blockTemplate.Transactions {
		copy(txHashes[i+1][:], util.ReverseBuffer(util.HexToBytes(transaction.Hash)))
	}

	var txMerkleTreeRootReversed [32]byte

	if len(txHashes) > 1 {
		txMerkleTree := merkleTree.NewMerkleTree(txHashes)
		txMerkleTreeRootReversed = txMerkleTree.MerkleRoot()
	} else {
		copy(txMerkleTreeRootReversed[:], txHashes[0][:])
	}

	target, _ := new(big.Int).SetString(blockTemplate.Target, 16)
	newWork := Work{
		JobId:                util.GetHexTimestamp(),
		Version:              util.BytesToHex(util.PackUInt32LE(blockTemplate.Version)),
		PrevHashReversed:     util.ReverseHex(blockTemplate.PrevBlockHash),
		MerkleRootReversed:   util.BytesToHex(txMerkleTreeRootReversed[:]),
		FinalSaplingRootHash: util.ReverseHex(blockTemplate.FinalSaplingRootHash),
		Time:                 util.BytesToHex(util.PackUInt32LE(blockTemplate.CurTime)),
		Bits:                 util.ReverseHex(blockTemplate.Bits),
		Target:               blockTemplate.Target,
		Height:               blockTemplate.Height,
		Difficulty:           new(big.Int).Div(util.PowLimitTest, target),
		CleanJobs:            true,
		Template:             &blockTemplate,
		GeneratedCoinbase:    coinbaseTxn,
		FeeReward:            feeReward,
	}

	proxyServer.work.Store(&newWork)
	log.Printf("New block to mine on %s at height %d", rpc.Name, blockTemplate.Height)

	// Stratum
	if proxyServer.config.Proxy.Stratum.Enabled {
		go proxyServer.broadcastNewJobs()
	}
}

func (work *Work) BuildHeader(noncePart1, noncePart2 string) []byte {
	result := util.HexToBytes(work.Version)
	result = append(result, util.HexToBytes(work.PrevHashReversed)...)
	result = append(result, util.HexToBytes(work.MerkleRootReversed)...)
	result = append(result, util.HexToBytes(work.FinalSaplingRootHash)...)
	result = append(result, util.HexToBytes(work.Time)...)
	result = append(result, util.HexToBytes(work.Bits)...)
	result = append(result, util.HexToBytes(noncePart1)...)
	result = append(result, util.HexToBytes(noncePart2)...)
	return result
}

func (work *Work) CreateJob() []interface{} {
	return []interface{}{
		work.JobId,
		work.Version,
		work.PrevHashReversed,
		work.MerkleRootReversed,
		work.FinalSaplingRootHash,
		work.Time,
		work.Bits,
		work.CleanJobs,
	}
}
