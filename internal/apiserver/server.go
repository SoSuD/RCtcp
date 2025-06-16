package apiserver

import (
	"RCGOTCP/pkg/requestToTCP"
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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
	logger *zap.Logger
}

func newServer() *server {
	logger, err := zap.NewDevelopment() //todo: add config for zap

	if err != nil {
		panic(err)
	}
	return &server{
		router: gin.Default(),
		logger: logger,
	}
}

func (s *server) configureRouter() {
	s.router.Use(gin.Recovery())
	s.router.Any("/multiTCP", s.multiTCP)
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
		s.logger.Warn("failed to parse query", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, err := url.Parse(tcp.NextUrl)
	if err != nil {
		s.logger.Warn("failed to parse url", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.logger.Info("parsed url", zap.String("url", u.Host))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reschan := make(chan multiTCPResp, tcp.Count)
	request := c.Request
	request.URL.Path = u.Path
	req, err := requestToTCP.Rtotcp(request)
	if err != nil {
		s.logger.Warn("failed to parse request", zap.Error(err))
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
			s.logger.Debug("connect", zap.String("addr", u.Host), zap.Error(err))

			if err != nil {
				s.logger.Error("failed to connect", zap.Error(err))
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
				s.logger.Error("failed to tls handshake", zap.Error(err))
				reschan <- multiTCPResp{Err: err.Error()}
				wgsend.Done()
				return
			}
			if _, err := tlsConn.Write([]byte(reqWithoutChar)); err != nil {
				s.logger.Error("failed to send request part 1", zap.Error(err))
				reschan <- multiTCPResp{Err: err.Error()}
				return
			}
			s.logger.Debug("sent request part 1", zap.Int("count", i))
			wgsend.Done()
			wgsend.Wait()
			if _, err := tlsConn.Write([]byte(reqLastChar)); err != nil {
				s.logger.Error("failed to send request part 2", zap.Error(err))
				reschan <- multiTCPResp{Err: err.Error()}
				return
			}
			runtime.Gosched()
			s.logger.Debug("sent request part 2", zap.Int("count", i))

			br := bufio.NewReader(tlsConn) // ① буфер для чтения
			resp, err := http.ReadResponse(br, nil)
			// ② парсим HTTP-ответ
			if err != nil {
				s.logger.Error("failed to read response", zap.Error(err))
				reschan <- multiTCPResp{Err: err.Error()}
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				s.logger.Error("failed to read response body", zap.Error(err))
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
