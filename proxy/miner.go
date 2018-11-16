package proxy

import (
	"fmt"
	"log"
	"math/big"

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
		// fmt.Println(util.BytesToHex(header))

		var blockHex []byte = nil

		if HeaderLeTarget(header, work.Target) {
			fmt.Println("\n\n\n\nHEADER LOWER THAN TARGET!!!!!!!!!1111111111\n\n\n\n")

			blockHex = append(header, util.HexToBytes("01")...)
			blockHex = append(blockHex, util.HexToBytes(work.Template.CoinbaseTxn.Data)...)
		} else {
			if !SdiffDivDiffGe1(header, work) {
				fmt.Println("\n\n\nLow difficulty share\n\n\n")
				return false, &ErrorReply{Code: 23, Message: "Low difficulty share"}
			}
		}

		if blockHex != nil {
			_, err := s.rpc().SubmitBlock(util.BytesToHex(blockHex))
			if err != nil {
				fmt.Println("submitBlockError: ", err)
				// log.Printf("Block submission failure")
				return false, &ErrorReply{Code: 23, Message: "Suubmit block error"}
				// } else if !ok {
				// log.Printf("Block rejected")
				// return false, &ErrorReply{Code: 23, Message: "Invalid share"}
			} else {
				s.fetchWork()
				// exist, err := s.backend.WriteBlock(login, id, params, shareDiff, h.diff.Int64(), h.height, s.hashrateExpiration)
				// if exist {
				// 	return true, false
				// }
				// if err != nil {
				// 	log.Println("Failed to insert block candidate into backend:", err)
				// } else {
				// 	log.Printf("Inserted block %v to backend", h.height)
				// }
				fmt.Println("Block found by miner %v@%v at height", cs.login, cs.ip)
				return true, nil
			}
		}

		log.Printf(" Share found by miner %v@%v at height %v", cs.login, cs.ip, work.Height)
		return true, nil

	} else {
		fmt.Println("Equihash verify not ok")
		// exist, err := s.backend.WriteShare(login, id, params, shareDiff, h.height, s.hashrateExpiration)
		// if exist {
		// 	return true, false
		// }
		// if err != nil {
		// 	log.Println("Failed to insert share data into backend:", err)
		// }
		return false, &ErrorReply{Code: 23, Message: "Equihash verify not ok"}
	}
	// shareExists, validShare, errorReply := s.processShare(cs, id, t, params)
	// ok := s.policy.ApplySharePolicy(cs.ip, !shareExists && validShare)

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
