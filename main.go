package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	clients   = make(map[*websocket.Conn]bool) // 연결된 클라이언트
	broadcast = make(chan RequestMessage)      // 메시지 브로드캐스트 채널
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	mu sync.Mutex
)

// RequestMessage 구조체
type RequestMessage struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

func main() {
	// Gin 설정 - release 모드로 전환
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// WebSocket 핸들러 설정
	router.GET("/ws", handleConnections)

	// 메시지를 브로드캐스트하는 고루틴 실행
	go handleMessages()

	// 서버 설정
	srv := &http.Server{
		Addr:    ":8082",
		Handler: router,
	}

	// 서버 시작
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Sever start failed: %v", err)
		}
	}()
	log.Println("Sever Start")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Sever quting...")

	// Graceful Shutdown: 타임아웃 5초
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Enternal Sever Error:", err)
	}

	log.Println("Sever close Normally.")
}

// WebSocket 연결을 처리하는 핸들러
func handleConnections(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Fatal("WebSocket join fail:", err)
	}
	defer ws.Close()

	// 새로운 클라이언트 등록
	mu.Lock()
	clients[ws] = true
	mu.Unlock()

	log.Println("WebSocket join:", ws.RemoteAddr())

	// 클라이언트로부터 메시지를 계속 수신
	for {
		var reqMsg RequestMessage
		// JSON 메시지 수신
		err := ws.ReadJSON(&reqMsg)
		if err != nil {
			log.Printf("Message read error: %v", err)
			mu.Lock()
			delete(clients, ws)
			mu.Unlock()
			break
		}

		// 수신한 메시지를 브로드캐스트 채널로 전송
		broadcast <- reqMsg
	}
}

// 브로드캐스트 메시지를 처리하는 함수
func handleMessages() {
	for {
		// 브로드캐스트 채널에서 메시지 수신
		msg := <-broadcast

		// 메시지를 단일 문자열로 변환 (예: "Username: message")
		broadcastMsg := msg.Username + ": " + msg.Message

		// 모든 클라이언트에 메시지 전송
		mu.Lock()
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, []byte(broadcastMsg))
			if err != nil {
				log.Printf("Message send failed: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
		mu.Unlock()
	}
}
