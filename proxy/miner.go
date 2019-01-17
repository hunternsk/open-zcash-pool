package proxy

import (
	"fmt"
	"log"
	"math/big"
	"strconv"

	"github.com/jkkgbe/open-zcash-pool/equihash"
	"github.com/jkkgbe/open-zcash-pool/util"
)

func (s *ProxyServer) processShare(cs *Session, id string, params []string) (bool, *ErrorReply) {
	// workerId := params[0]
	// jobId := params[1]
	// nTime := params[2]
	extraNonce2 := params[3]
	solution := params[4]

	work := s.currentWork()
	header := work.BuildHeader(cs.extraNonce1, extraNonce2)

	ok, err := equihash.Verify(200, 9, header, util.HexToBytes(solution)[3:])
	if err != nil {
		fmt.Println("equihashVerify error: ", err)
	}
	if ok {
		header = append(header, util.HexToBytes(solution)...)

		var blockHex []byte = nil

		if HeaderLeTarget(header, work.Target) {
			log.Println("Found block candidate")

			txCountAsHex := strconv.FormatInt(int64(len(work.Template.Transactions)+1), 16)

			if len(txCountAsHex)%2 == 1 {
				txCountAsHex = "0" + txCountAsHex
			}

			blockHex = append(header, util.HexToBytes(txCountAsHex)...)
			blockHex = append(blockHex, work.GeneratedCoinbase...)

			for _, transaction := range work.Template.Transactions {
				blockHex = append(blockHex, util.HexToBytes(transaction.Data)...)
			}
		} else {
			if !SdiffDivDiffGe1(header, work) {
				fmt.Println("Low difficulty share")
				return false, &ErrorReply{Code: 23, Message: "Low difficulty share"}
			}
		}

		if blockHex != nil {
			reply, err := s.rpc().SubmitBlock(util.BytesToHex(blockHex))
			if err != nil {
				fmt.Println("submitBlockError: ", err, reply)
				// log.Printf("Block submission failure")
				return false, &ErrorReply{Code: 23, Message: "Submit block error"}
				// } else if !ok {
				// log.Printf("Block rejected")
				// return false, &ErrorReply{Code: 23, Message: "Invalid share"}
				_, err := s.backend.WriteShare(cs.login, id, params, s.config.Proxy.Difficulty, work.Height, s.hashrateExpiration)
				if err != nil {
					log.Println("Failed to insert share data into backend:", err)
				}
			} else {
				log.Printf("Block found by miner %v@%v at height %v, id %v", cs.login, cs.ip, work.Height, reply)
				s.fetchWork()
				shareDiff := s.config.Proxy.Difficulty
				exist, err := s.backend.WriteBlock(cs.login, id, params, shareDiff, work.Difficulty.Int64(), work.Height, s.hashrateExpiration)
				if exist {
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

		log.Printf(" Share found by miner %v@%v at height %v", cs.login, cs.ip, work.Height)
		return true, nil

	} else {
		fmt.Println("Equihash verify not ok")
		return false, &ErrorReply{Code: 23, Message: "Equihash verify not ok"}
	}
	// shareExists, validShare, errorReply := s.processShare(cs, id, t, params)

	// if !validShare {
	// 	log.Printf("Invalid share from %s@%s", cs.login, cs.ip)
	// 	// Bad shares limit reached, return error and close
	// 	if !ok {
	// 		return false, false, errorReply
	// 	}
	// 	return false, false, nil
	// }
	// log.Printf("Valid share from %s@%s", cs.login, cs.ip)

	// if shareExists {
	// 	log.Printf("Duplicate share from %s@%s %v", cs.login, cs.ip, params)
	// 	return false, false, &ErrorReply{Code: 22, Message: "Duplicate share"}
	// }

	// if !ok {
	// 	return false, true, &ErrorReply{Code: -1, Message: "High rate of invalid shares"}
	// }
	// return false, true, nil
}

// func HashHeader(w *Work, header []byte) (ShareStatus, string) {
// 	round1 := sha256.Sum256(header)
// 	round2 := sha256.Sum256(round1[:])

// 	round2 = util.ReverseBuffer(round2[:])

// 	// Check against the global target
// 	if TargetCompare(round2, w.Template.Target) <= 0 {
// 		return ShareBlock, hex.EncodeToString(round2[:])
// 	}

// 	if TargetCompare(round2, shareTarget) > 1 {
// 		return ShareInvalid, ""
// 	}

// 	return ShareOK, ""
// }

func SdiffDivDiffGe1(header []byte, work *Work) bool {
	headerHashed := util.Sha256d(header)

	headerBig := new(big.Int).SetBytes(util.ReverseBuffer(headerHashed[:]))
	shareDifficulty := new(big.Rat).SetFrac(util.PowLimitTest, headerBig)
	ratCmp := new(big.Rat).Quo(shareDifficulty, new(big.Rat).SetInt64(32)).Cmp(new(big.Rat).SetInt64(1))
	diffOk := ratCmp >= 0

	return diffOk
}

func HeaderLeTarget(header []byte, target string) bool {
	headerHashed := util.Sha256d(header)

	x := new(big.Int).SetBytes(util.ReverseBuffer(headerHashed[:]))
	y, _ := new(big.Int).SetString(target, 16)
	bol := x.Cmp(y) <= 0
	return bol
}
