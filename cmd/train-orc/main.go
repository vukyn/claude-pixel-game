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
	"syscall"

	"claude-pixel/internal/aienv"
)

func main() {
	port := flag.Int("port", 9876, "listen port")
	flag.Parse()

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("listen on port %d: %v", *port, err)
	}
	log.Printf("orc training server listening on :%d", *port)

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("shutting down...")
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			return
		}
		log.Println("client connected")
		handleOrcConn(conn)
		conn.Close()
		log.Println("client disconnected")
	}
}

func handleOrcConn(conn net.Conn) {
	env, err := aienv.NewOrcTrainEnv(aienv.OrcEnvConfig{Seed: 1})
	if err != nil {
		log.Printf("create OrcTrainEnv: %v", err)
		return
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("read: %v", err)
			}
			return
		}

		var msg OrcClientMsg
		if err := json.Unmarshal(line, &msg); err != nil {
			log.Printf("unmarshal: %v", err)
			continue
		}

		var resp OrcObsMsg
		switch msg.Type {
		case "reset":
			result := env.Reset()
			resp = OrcObsMsg{
				Type: "obs", PlayerObs: result.PlayerObs, PlayerReward: result.PlayerReward,
				OrcObs: result.OrcObs, OrcRewards: result.OrcRewards,
				OrcDones: result.OrcDones, Done: result.Done, Info: result.Info,
			}
		case "action":
			result := env.Step(msg.PlayerAction, msg.OrcActions)
			resp = OrcObsMsg{
				Type: "obs", PlayerObs: result.PlayerObs, PlayerReward: result.PlayerReward,
				OrcObs: result.OrcObs, OrcRewards: result.OrcRewards,
				OrcDones: result.OrcDones, Done: result.Done, Info: result.Info,
			}
		default:
			log.Printf("unknown message type: %q", msg.Type)
			continue
		}

		data, _ := json.Marshal(resp)
		writer.Write(data)
		writer.WriteByte('\n')
		writer.Flush()
	}
}
