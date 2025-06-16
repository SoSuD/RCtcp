package apiserver

import (
	"RCGOTCP/pkg/requestToTCP"
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"time"
)

type server struct {
	router *gin.Engine
}

func newServer() *server {
	return &server{
		router: gin.Default(),
	}
}

func (s *server) configureRouter() {
	s.router.Use(gin.Recovery())
	s.router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %d %d %s \"%s\" \"\n",
			param.TimeStamp.Format(time.RFC822),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage)
	}))
	s.router.Any("/simpleTCP", s.simpleTCP)
	s.router.Any("/multiTCP", s.multiTCP)
}

type simpleTCPReq struct {
	NextUrl string `form:"next_url"`
	Port    string `form:"port"`
	Count   int    `form:"count"`
}

func (s *server) simpleTCP(c *gin.Context) {
	var tcp simpleTCPReq
	if err := c.ShouldBindQuery(&tcp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := url.Parse(tcp.NextUrl)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%s", u.Host, tcp.Port))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer conn.Close()

	tlsConn := tls.Client(conn, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         u.Host,
	})
	defer tlsConn.Close()
	tlsConn.Handshake()

	request := c.Request
	request.URL.Path = u.Path

	req, err := requestToTCP.Rtotcp(request)
	fmt.Println(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := tlsConn.Write([]byte(req)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	br := bufio.NewReader(tlsConn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)

}

type multiTCPReq struct {
	NextUrl string `form:"next_url"`
	Port    string `form:"port"`
	Count   int    `form:"count"`
}

type multiTCPResp struct {
	Response string `json:"response"`
	Err      string `json:"error,omitempty"`
}

func (s *server) multiTCP(c *gin.Context) {
	var tcp multiTCPReq
	if err := c.ShouldBindQuery(&tcp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, err := url.Parse(tcp.NextUrl)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reschan := make(chan multiTCPResp, tcp.Count)
	request := c.Request
	request.URL.Path = u.Path
	req, err := requestToTCP.Rtotcp(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	wgsend := new(sync.WaitGroup)
	wgsend.Add(tcp.Count)

	wgrecv := new(sync.WaitGroup)
	wgrecv.Add(tcp.Count)

	reqWithoutChar := req[0 : len(req)-2]
	reqLastChar := req[len(req)-2:]

	for i := 0; i < tcp.Count; i++ {
		go func() {
			defer wgrecv.Done()
			dialer := &net.Dialer{}
			conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%s", u.Host, tcp.Port))

			if err != nil {
				reschan <- multiTCPResp{Err: err.Error()}
				wgsend.Done()
				return
			}
			defer conn.Close()
			tlsConn := tls.Client(conn, &tls.Config{
				ServerName: u.Host,
			})
			defer tlsConn.Close()

			if err := tlsConn.Handshake(); err != nil {
				reschan <- multiTCPResp{Err: err.Error()}
				wgsend.Done()
				return
			}
			if _, err := tlsConn.Write([]byte(reqWithoutChar)); err != nil {
				reschan <- multiTCPResp{Err: err.Error()}
				return
			}
			wgsend.Done()
			wgsend.Wait()
			if _, err := tlsConn.Write([]byte(reqLastChar)); err != nil {
				reschan <- multiTCPResp{Err: err.Error()}
				return
			}
			fmt.Println(time.Now().UnixNano())
			runtime.Gosched()

			br := bufio.NewReader(tlsConn) // ① буфер для чтения
			resp, err := http.ReadResponse(br, nil)
			// ② парсим HTTP-ответ
			if err != nil {
				reschan <- multiTCPResp{Err: err.Error()}
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				reschan <- multiTCPResp{Err: err.Error()}
				return
			}
			reschan <- multiTCPResp{Response: string(body)}

		}()
	}
	wgrecv.Wait()
	close(reschan)

	responses := make([]multiTCPResp, tcp.Count)
	for i := 0; i < tcp.Count; i++ {
		responses[i] = <-reschan

	}

	c.JSON(200, gin.H{"responses": responses})

}
