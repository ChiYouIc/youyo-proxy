package web

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebsocketConnection struct {
	conn *websocket.Conn
}

func (wc *WebsocketConnection) handleConnection(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	handleConnErr(err)
	//
	//if wc.conn != nil {
	//	_ = wc.conn.WriteMessage(websocket.TextMessage, bytes.NewBufferString("断开连接").Bytes())
	//	_ = wc.conn.Close()
	//}

	wc.conn = conn

	for {
		// 接受消息
		message, p, err := conn.ReadMessage()
		handleConnErr(err)
		fmt.Printf("messageType: %d, message: %s", message, p)
	}
}

func (wc *WebsocketConnection) WriteMessage(msg string) {

	if wc.conn == nil {
		fmt.Println("没有任何主机接入")
		return
	}

	err := wc.conn.WriteMessage(websocket.TextMessage, bytes.NewBufferString(msg).Bytes())
	handleConnErr(err)
}

var WC = &WebsocketConnection{}

func Router() http.Handler {
	e := gin.New()
	e.Use(gin.Recovery())
	e.GET("/ws", WC.handleConnection)
	return e
}

func handleConnErr(err error) {
	if err != nil {
		panic(err)
	}
}
