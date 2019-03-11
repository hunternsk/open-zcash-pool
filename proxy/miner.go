package proxy

import (
	"log"
	"math/big"
	"strconv"

	"github.com/jkkgbe/open-zcash-pool/equihash"
	"github.com/jkkgbe/open-zcash-pool/util"
)

func (proxyServer *ProxyServer) processShare(session *Session, id string, params []string) (bool, *ErrorReply) {
	extraNonce2 := params[3]
	solution := params[4]

	work := proxyServer.currentWork()
	header := work.BuildHeader(session.extraNonce1, extraNonce2)

	headerWithSol := append(header, util.HexToBytes(solution)...)

	var blockHex []byte = nil

	if isHeaderLeTarget(headerWithSol, work.Target) {
		txCountAsHex := strconv.FormatInt(int64(len(work.Template.Transactions)+1), 16)

		if len(txCountAsHex)%2 == 1 {
			txCountAsHex = "0" + txCountAsHex
		}

		blockHex = append(headerWithSol, util.HexToBytes(txCountAsHex)...)
		blockHex = append(blockHex, work.GeneratedCoinbase...)

		for _, transaction := range work.Template.Transactions {
			blockHex = append(blockHex, util.HexToBytes(transaction.Data)...)
		}
	} else {
		if !isShareDiffGeDiff(headerWithSol, proxyServer.config.Proxy.Difficulty) {
			return false, &ErrorReply{Code: 23, Message: "Low difficulty share"}
		}
	}

	ok, err := equihash.Verify(200, 9, header, util.HexToBytes(solution)[3:])
	if err != nil {
		log.Println("Equihash verifier error:", err)
	}
	if ok {
		if blockHex != nil {
			reply, err := proxyServer.rpc().SubmitBlock(util.BytesToHex(blockHex))
			if err != nil {
				return false, &ErrorReply{Code: 23, Message: "Submit block error"}
			} else {
				log.Printf("Block found by miner %v@%v at height %v, id %v", session.login, session.ip, work.Height, reply)
				proxyServer.fetchWork()
				shareDiff := proxyServer.config.Proxy.Difficulty
				blockHash := util.Sha256d(headerWithSol)
				exists, err := proxyServer.backend.WriteBlock(session.login, id, params, shareDiff, work.Difficulty.Int64(), work.Height, proxyServer.hashrateExpiration, work.FeeReward, util.BytesToHex(util.ReverseBuffer(blockHash[:])))

				if exists {
					return true, nil
				}

				if err != nil {
					log.Println("Failed to insert block candidate into backend:", err)
				} else {
					log.Printf("Inserted block %v to backend", work.Height)
				}

				return true, nil
			}
		}

		_, err := proxyServer.backend.WriteShare(session.login, id, params, proxyServer.config.Proxy.Difficulty, work.Height, proxyServer.hashrateExpiration)
		if err != nil {
			log.Println("Failed to insert share data into backend:", err)
		}

		log.Printf("Share found by miner %v@%v at height %v", session.login, session.ip, work.Height)

		return true, nil
	} else {
		return false, &ErrorReply{Code: 23, Message: "Incorrect solution"}
	}
}

func isShareDiffGeDiff(header []byte, minerDifficulty int64) bool {
	headerHashed := util.Sha256d(header)
	headerBig := new(big.Int).SetBytes(util.ReverseBuffer(headerHashed[:]))
	shareDifficulty := new(big.Rat).SetFrac(util.PowLimitTest, headerBig)
	ratCmp := new(big.Rat).Quo(shareDifficulty, new(big.Rat).SetInt64(minerDifficulty)).Cmp(new(big.Rat).SetInt64(1))
	diffOk := ratCmp >= 0

	return diffOk
}

func isHeaderLeTarget(header []byte, target string) bool {
	headerHashed := util.Sha256d(header)

	x := new(big.Int).SetBytes(util.ReverseBuffer(headerHashed[:]))
	y, _ := new(big.Int).SetString(target, 16)
	bol := x.Cmp(y) <= 0

	return bol
}
