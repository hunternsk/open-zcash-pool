package proxy

import (
	"log"
	"regexp"

	"github.com/jkkgbe/open-zcash-pool/util"
)

// Allow only lowercase hexadecimal with 0x prefix
var nTimePattern = regexp.MustCompile("^[0-9a-f]{8}$")
var noncePattern = regexp.MustCompile("^[0-9a-f]{64}$")

var workerPattern = regexp.MustCompile("^[0-9a-zA-Z-_]{1,8}$")

func (proxyServer *ProxyServer) handleSubscribeRPC(session *Session, extraNonce1 string) []string {
	session.extraNonce1 = extraNonce1
	array := []string{"0", extraNonce1}
	return array
}

func (proxyServer *ProxyServer) handleAuthorizeRPC(session *Session, params []string) (bool, *ErrorReply) {
	if len(params) == 0 {
		return false, &ErrorReply{Code: -1, Message: "Invalid params"}
	}

	login := params[0]
	if !util.IsValidLogin(login) {
		return false, &ErrorReply{Code: -1, Message: "Invalid login"}
	}
	session.login = login
	proxyServer.registerSession(session)
	log.Printf("Stratum miner connected %v@%v", login, session.ip)
	return true, nil
}

func (proxyServer *ProxyServer) handleTCPSubmitRPC(session *Session, params []string, id string) (bool, *ErrorReply) {
	proxyServer.sessionsMu.RLock()
	_, ok := proxyServer.sessions[session]
	proxyServer.sessionsMu.RUnlock()

	if !ok {
		return false, &ErrorReply{Code: 24, Message: "Not authorized"}
	}
	if session.extraNonce1 == "" {
		return false, &ErrorReply{Code: 25, Message: "Not subscribed"}
	}
	return proxyServer.handleSubmitRPC(session, params, id)
}

func (proxyServer *ProxyServer) handleSubmitRPC(session *Session, params []string, id string) (bool, *ErrorReply) {
	if !workerPattern.MatchString(id) {
		id = "0"
	}

	if len(params) != 5 {
		log.Printf("Malformed params from %s@%s %v", session.login, session.ip, params)
		return false, &ErrorReply{Code: -1, Message: "Invalid params"}
	}

	if !nTimePattern.MatchString(params[2]) {
		log.Printf("Malformed nTime result from %s@%s %v", session.login, session.ip, params)
		return false, &ErrorReply{Code: -1, Message: "Malformed nTime result"}
	}

	if !noncePattern.MatchString(session.extraNonce1 + params[3]) {
		log.Printf("Malformed nonce result from %s@%s %v", session.login, session.ip, params)
		return false, &ErrorReply{Code: -1, Message: "Malformed nonce result"}
	}

	if len(params[4]) != 2694 {
		log.Printf("Malformed solution result from %s@%s %v", session.login, session.ip, params)
		return false, &ErrorReply{Code: -1, Message: "Malformed solution result, != 2694 length"}
	}

	return proxyServer.processShare(session, id, params)
}

func (proxyServer *ProxyServer) handleUnknownRPC(session *Session, method string) *ErrorReply {
	log.Printf("Unknown request method %s from %s", method, session.ip)
	return &ErrorReply{Code: -3, Message: "Method not found"}
}
