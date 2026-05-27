package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"claude-pixel/internal/aienv"
)

func main() {
	numEnvs := flag.Int("envs", 4, "number of parallel environments")
	basePort := flag.Int("port", 9876, "base port (env i listens on port+i)")
	flag.Parse()

	var wg sync.WaitGroup
	listeners := make([]net.Listener, *numEnvs)

	for i := 0; i < *numEnvs; i++ {
		port := *basePort + i
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			log.Fatalf("env %d: listen on port %d: %v", i, port, err)
		}
		listeners[i] = ln
		log.Printf("env %d listening on :%d", i, port)

		wg.Add(1)
		go func(id int, ln net.Listener) {
			defer wg.Done()
			serveEnv(id, ln)
		}(i, ln)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down...")
	for _, ln := range listeners {
		ln.Close()
	}
	wg.Wait()
}

func serveEnv(id int, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("env %d: accept: %v", id, err)
			return
		}
		log.Printf("env %d: client connected", id)
		handleConn(id, conn)
		conn.Close()
		log.Printf("env %d: client disconnected", id)
	}
}

func handleConn(id int, conn net.Conn) {
	env, err := aienv.NewGameEnv(aienv.EnvConfig{Seed: int64(id + 1)})
	if err != nil {
		log.Printf("env %d: create GameEnv: %v", id, err)
		return
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("env %d: read: %v", id, err)
			}
			return
		}

		var msg ClientMsg
		if err := json.Unmarshal(line, &msg); err != nil {
			log.Printf("env %d: unmarshal: %v", id, err)
			continue
		}

		var resp ObsMsg
		switch msg.Type {
		case "reset":
			obs := env.Reset()
			resp = ObsMsg{Type: "obs", Obs: obs, Reward: 0, Done: false}

		case "action":
			obs, reward, done, info := env.Step(msg.Action)
			resp = ObsMsg{Type: "obs", Obs: obs, Reward: reward, Done: done, Info: info}

		default:
			log.Printf("env %d: unknown message type: %q", id, msg.Type)
			continue
		}

		data, _ := json.Marshal(resp)
		writer.Write(data)
		writer.WriteByte('\n')
		writer.Flush()
	}
}
