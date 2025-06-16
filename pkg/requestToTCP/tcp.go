package requestToTCP

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// POST /api/v1/users HTTP/1.1\r\n
// Host: example.com\r\n
// User-Agent: raw-tcp/1.0\r\n
// Content-Type: application/json\r\n
// Content-Length: 22\r\n
// Connection: close\r\n
// \r\n
// {"name":"Alice","age":30}
func Rtotcp(req *http.Request) (string, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s %s %s\r\n"+
		"%s\r\n"+
		"%s", req.Method, req.URL.Path, req.Proto, htos(req.Header, req.Host), string(body)), nil
}

func htos(header http.Header, host string) string {
	var buffer bytes.Buffer
	for key, values := range header {
		for _, value := range values {
			fmt.Fprintf(&buffer, "%s: %s\r\n", key, value)
		}
	}
	fmt.Fprintf(&buffer, "%s: %s\r\n", "Host", host)
	return buffer.String()
}
