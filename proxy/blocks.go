package proxy

import (
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/jkkgbe/open-zcash-pool/merkleTree"
	"github.com/jkkgbe/open-zcash-pool/transaction"
	"github.com/jkkgbe/open-zcash-pool/util"
)

const maxBacklog = 3

type heightDiffPair struct {
	diff   *big.Int
	height uint64
}

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
	Height               uint64        `json:"height"`
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
	Height               uint64
	Difficulty           *big.Int
	CleanJobs            bool
	Template             *BlockTemplate
	GeneratedCoinbase    []byte
}

func (s *ProxyServer) fetchWork() {
	rpc := s.rpc()
	t := s.currentWork()
	var reply BlockTemplate
	err := rpc.GetBlockTemplate(&reply)
	if err != nil {
		log.Printf("Error while refreshing block template on %s: %s", rpc.Name, err)
		return
	}
	// No need to update, we have fresh job
	if t != nil && util.BytesToHex(util.ReverseBuffer(util.HexToBytes(t.PrevHashReversed))) == reply.PrevBlockHash {
		return
	}

	var feeReward int64 = 0
	for _, transaction := range reply.Transactions {
		feeReward += transaction.Fee
	}

	coinbaseTxn, coinbaseHash := transaction.BuildCoinbaseTxn(reply.Height, s.config.PoolAddress, reply.CoinbaseTxn.FoundersReward, feeReward)

	txHashes := make([][32]byte, len(reply.Transactions)+1)
	log.Println("CBTX HASH: ", util.BytesToHex(coinbaseHash[:]))
	copy(txHashes[0][:], coinbaseHash[:])
	for i, transaction := range reply.Transactions {
		log.Println("TX HASH: ", transaction.Hash)
		copy(txHashes[i+1][:], util.ReverseBuffer(util.HexToBytes(transaction.Hash)))
	}

	var mtr [32]byte

	if len(txHashes) > 1 {
		mt := merkleTree.NewMerkleTree(txHashes)
		mtr = mt.MerkleRoot()
	} else {
		copy(mtr[:], txHashes[0][:])
	}

	target, _ := new(big.Int).SetString(reply.Target, 16)
	log.Println("MTR: ", util.BytesToHex(mtr[:]))
	newWork := Work{
		JobId:                util.BytesToHex([]byte(time.Now().String())),
		Version:              util.BytesToHex(util.PackUInt32LE(reply.Version)),
		PrevHashReversed:     util.BytesToHex(util.ReverseBuffer(util.HexToBytes(reply.PrevBlockHash))),
		MerkleRootReversed:   util.BytesToHex(mtr[:]),
		FinalSaplingRootHash: util.BytesToHex(util.ReverseBuffer(util.HexToBytes(reply.FinalSaplingRootHash))),
		Time:                 util.BytesToHex(util.PackUInt32LE(reply.CurTime)),
		Bits:                 util.BytesToHex(util.ReverseBuffer(util.HexToBytes(reply.Bits))),
		Target:               reply.Target,
		Height:               reply.Height,
		Difficulty:           new(big.Int).Div(util.PowLimitTest, target),
		CleanJobs:            true,
		Template:             &reply,
		GeneratedCoinbase:    coinbaseTxn,
	}
	log.Println("MTRR: ", newWork.MerkleRootReversed)

	s.work.Store(&newWork)
	log.Printf("New block to mine on %s at height %d", rpc.Name, reply.Height)

	// Stratum
	if s.config.Proxy.Stratum.Enabled {
		go s.broadcastNewJobs()
	}
}

func (w *Work) BuildHeader(noncePart1, noncePart2 string) []byte {
	result := util.HexToBytes(w.Version)
	result = append(result, util.HexToBytes(w.PrevHashReversed)...)
	result = append(result, util.HexToBytes(w.MerkleRootReversed)...)
	result = append(result, util.HexToBytes(w.FinalSaplingRootHash)...)
	result = append(result, util.HexToBytes(w.Time)...)
	result = append(result, util.HexToBytes(w.Bits)...)
	result = append(result, util.HexToBytes(noncePart1)...)
	result = append(result, util.HexToBytes(noncePart2)...)
	return result
}

func (w *Work) CreateJob() []interface{} {
	return []interface{}{
		w.JobId,
		w.Version,
		w.PrevHashReversed,
		w.MerkleRootReversed,
		w.FinalSaplingRootHash,
		w.Time,
		w.Bits,
		w.CleanJobs,
	}
}
