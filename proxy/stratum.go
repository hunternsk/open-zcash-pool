package proxy

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net"
	"time"

	"github.com/jkkgbe/open-zcash-pool/util"
)

const (
	MaxReqSize = 10240
)

func (proxyServer *ProxyServer) ListenTCP() {
	timeout := util.MustParseDuration(proxyServer.config.Proxy.Stratum.Timeout)
	proxyServer.timeout = timeout

	addr, err := net.ResolveTCPAddr("tcp", proxyServer.config.Proxy.Stratum.Listen)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	server, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer server.Close()

	log.Printf("Stratum listening on %s", proxyServer.config.Proxy.Stratum.Listen)
	var accept = make(chan int, proxyServer.config.Proxy.Stratum.MaxConn)
	n := 0

	for {
		conn, err := server.AcceptTCP()
		if err != nil {
			continue
		}
		conn.SetKeepAlive(true)

		ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

		n += 1
		session := &Session{conn: conn, ip: ip}

		accept <- n
		go func(session *Session) {
			err = proxyServer.handleTCPClient(session)
			if err != nil {
				proxyServer.removeSession(session)
				conn.Close()
			}
			<-accept
		}(session)
	}
}

func (proxyServer *ProxyServer) handleTCPClient(session *Session) error {
	session.enc = json.NewEncoder(session.conn)
	connbuff := bufio.NewReaderSize(session.conn, MaxReqSize)
	proxyServer.setDeadline(session.conn)

	for {
		data, isPrefix, err := connbuff.ReadLine()
		if isPrefix {
			log.Printf("Socket flood detected from %s", session.ip)
			return err
		} else if err == io.EOF {
			log.Printf("Client %s disconnected", session.ip)
			proxyServer.removeSession(session)
			break
		} else if err != nil {
			log.Printf("Error reading from socket: %v", err)
			return err
		}

		if len(data) > 1 {
			var req StratumReq
			err = json.Unmarshal(data, &req)
			if err != nil {
				log.Printf("Malformed stratum request from %s: %v", session.ip, err)
				return err
			}
			proxyServer.setDeadline(session.conn)
			err = session.handleTCPMessage(proxyServer, &req)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (session *Session) handleTCPMessage(proxyServer *ProxyServer, req *StratumReq) error {
	var params []string
	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		log.Println("Malformed stratum request params from", session.ip)
		return err
	}

	var reply interface{}
	var errReply *ErrorReply
	// Handle RPC methods
	switch req.Method {
	case "mining.subscribe":
		extraNonce1 := proxyServer.nextExtraNonce1()
		reply = proxyServer.handleSubscribeRPC(session, extraNonce1)
	case "mining.authorize":
		reply, errReply = proxyServer.handleAuthorizeRPC(session, params)
		if errReply != nil {
			return session.sendTCPError(req.Id, errReply)
		}
		session.sendTCPResult(req.Id, reply)

		var diff = []interface{}{proxyServer.diff}
		session.setTarget(&diff)
		currentWork := proxyServer.currentWork()
		if currentWork == nil || proxyServer.isSick() {
			return nil
		}
		reply := currentWork.CreateJob()
		return session.pushNewJob(&reply)
	case "mining.submit":
		reply, errReply = proxyServer.handleTCPSubmitRPC(session, params, req.Worker)
	case "mining.extranonce.subscribe":
		errReply = &ErrorReply{Code: 20, Message: "Not supported."}
	default:
		errReply = proxyServer.handleUnknownRPC(session, req.Method)
	}

	if errReply != nil {
		return session.sendTCPError(req.Id, errReply)
	}

	return session.sendTCPResult(req.Id, reply)
}

func (session *Session) sendTCPResult(id json.RawMessage, result interface{}) error {
	session.Lock()
	defer session.Unlock()

	message := JSONRpcResp{Id: id, Version: "2.0", Error: nil, Result: result}
	return session.enc.Encode(&message)
}

func (session *Session) setTarget(params *[]interface{}) error {
	session.Lock()
	defer session.Unlock()
	message := JSONPushMessage{Version: "2.0", Method: "mining.set_target", Params: *params, Id: 0}
	return session.enc.Encode(&message)
}

func (session *Session) pushNewJob(params *[]interface{}) error {
	session.Lock()
	defer session.Unlock()
	message := JSONPushMessage{Version: "2.0", Method: "mining.notify", Params: *params, Id: 0}
	return session.enc.Encode(&message)
}

func (session *Session) sendTCPError(id json.RawMessage, reply *ErrorReply) error {
	session.Lock()
	defer session.Unlock()

	message := JSONRpcResp{Id: id, Version: "2.0", Error: reply}
	return session.enc.Encode(&message)
}

func (proxyServer *ProxyServer) setDeadline(conn *net.TCPConn) {
	conn.SetDeadline(time.Now().Add(proxyServer.timeout))
}

func (proxyServer *ProxyServer) registerSession(session *Session) {
	proxyServer.sessionsMu.Lock()
	defer proxyServer.sessionsMu.Unlock()
	proxyServer.sessions[session] = struct{}{}
}

func (proxyServer *ProxyServer) removeSession(session *Session) {
	proxyServer.sessionsMu.Lock()
	defer proxyServer.sessionsMu.Unlock()
	delete(proxyServer.sessions, session)
}

func (proxyServer *ProxyServer) broadcastNewJobs() {
	currentWork := proxyServer.currentWork()

	if currentWork == nil || proxyServer.isSick() {
		return
	}
	reply := currentWork.CreateJob()

	proxyServer.sessionsMu.RLock()
	defer proxyServer.sessionsMu.RUnlock()

	count := len(proxyServer.sessions)
	log.Printf("Broadcasting new job to %v stratum miners", count)

	start := time.Now()
	bcast := make(chan int, 1024)
	n := 0
	for miner := range proxyServer.sessions {
		n++
		bcast <- n

		go func(session *Session) {
			err := session.pushNewJob(&reply)
			<-bcast
			if err != nil {
				log.Printf("Job transmit error to %v@%v: %v", session.login, session.ip, err)
				proxyServer.removeSession(session)
			} else {
				proxyServer.setDeadline(session.conn)
			}
		}(miner)
	}
	log.Printf("Jobs broadcast finished %s", time.Since(start))
}
