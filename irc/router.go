package irc

import (
	"bufio"
	"encoding/gob"
	"net"
)

type RouterMessage struct {
	Id      string
	Message string
}

//
// router
//

type Router struct {
	connector net.Conn
	conns     map[string]*RouterConn
	decoder   *gob.Decoder
	encoder   *gob.Encoder
	writer    *bufio.Writer
	listener  net.Listener
}

func NewRouter() *Router {
	return &Router{
		conns: make(map[string]*RouterConn),
	}
}

func (router *Router) Connect(addr string) (err error) {
	if router.connector, err = net.Dial("tcp", addr); err != nil {
		return
	}
	router.decoder = gob.NewDecoder(bufio.NewReader(router.connector))
	router.writer = bufio.NewWriter(router.connector)
	router.encoder = gob.NewEncoder(router.writer)
	go router.ReadAll()
	return
}

func (router *Router) ReadAll() {
	for {
		msg, err := router.Read()
		if err != nil {
			Log.error.Println("Router.ReadAll:", err)
			break
		}
		rconn := router.conns[msg.Id]
		if rconn == nil {
			Log.warn.Println("Router.ReadAll: no such client:", msg.Id)
			continue
		}
		if err = rconn.Write(msg.Message); err != nil {
			Log.warn.Println("Router.ReadAll: write failed:", rconn)
			// TODO clean up rconn?
			continue
		}
	}
}

func (router *Router) Listen(addr string) (err error) {
	if router.listener, err = net.Listen("tcp", addr); err != nil {
		return
	}
	go func() {
		for {
			conn, err := router.listener.Accept()
			if err != nil {
				Log.error.Println("accept error", err)
				continue
			}
			Log.debug.Println("accept:", conn)
			rconn := NewRouterConn(conn)
			router.conns[rconn.Id()] = rconn
			go rconn.CopyTo(router)
		}
	}()
	return
}

func (router *Router) Read() (msg *RouterMessage, err error) {
	msg = &RouterMessage{}
	err = router.decoder.Decode(msg)
	return
}

func (router *Router) Write(rconn *RouterConn, message string) (err error) {
	err = router.encoder.Encode(RouterMessage{
		Id:      rconn.Id(),
		Message: message,
	})
	if err != nil {
		return
	}
	err = router.writer.Flush()
	return
}

//
// router connection
//

type RouterConn struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

func NewRouterConn(conn net.Conn) *RouterConn {
	rconn := &RouterConn{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
	}
	return rconn
}

func (rconn *RouterConn) CopyTo(router *Router) {
	for {
		line, err := rconn.reader.ReadString('\n')
		if err != nil {
			Log.debug.Printf("%s: error: %s", rconn, err)
			break
		}

		err = router.Write(rconn, line)
		if err != nil {
			Log.warn.Printf("%s: encode error: %s", rconn, err)
			break
		}

		Log.debug.Printf("%s: %s", rconn, line)
	}
}

func (rconn *RouterConn) Write(line string) (err error) {
	if _, err = rconn.writer.WriteString(line); err != nil {
		return
	}
	if err = rconn.writer.Flush(); err != nil {
		return
	}
	return
}

func (rconn *RouterConn) Id() string {
	return rconn.conn.LocalAddr().String()
}
