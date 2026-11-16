// osc-scan-api: minimal HTTP front-end for a local clamd, added for Eyevinn OSC.
// OSC only exposes HTTP(S), so this translates HTTP uploads into clamd INSTREAM.
//
//	GET  /         health (always 200; reports whether clamd answers PING)
//	GET  /version  clamd VERSION
//	POST /scan     multipart/form-data (every file part is scanned) or raw body
package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	clamdAddr = env("CLAMD_ADDR", "127.0.0.1:3310")
	maxBytes  = envInt("SCAN_MAX_BYTES", 100*1024*1024)
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func envInt(k string, d int64) int64 {
	if v, err := strconv.ParseInt(os.Getenv(k), 10, 64); err == nil && v > 0 {
		return v
	}
	return d
}

func dial() (net.Conn, error) {
	return net.DialTimeout("tcp", clamdAddr, 3*time.Second)
}

func command(cmd string) (string, error) {
	c, err := dial()
	if err != nil {
		return "", err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.Write([]byte("z" + cmd + "\x00")); err != nil {
		return "", err
	}
	b, err := io.ReadAll(c)
	return strings.TrimRight(string(b), "\x00\n"), err
}

type result struct {
	Filename  string `json:"filename,omitempty"`
	Status    string `json:"status"` // clean | infected
	Signature string `json:"signature,omitempty"`
	Bytes     int64  `json:"bytes"`
}

var errTooLarge = errors.New("too large")

// instream streams r to clamd using the INSTREAM protocol.
func instream(r io.Reader) (*result, error) {
	c, err := dial()
	if err != nil {
		return nil, err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(10 * time.Minute))
	if _, err := c.Write([]byte("zINSTREAM\x00")); err != nil {
		return nil, err
	}
	buf := make([]byte, 64*1024)
	var total int64
	hdr := make([]byte, 4)
	for {
		n, rerr := r.Read(buf)
		if n > 0 {
			total += int64(n)
			if total > maxBytes {
				return nil, errTooLarge
			}
			binary.BigEndian.PutUint32(hdr, uint32(n))
			if _, err := c.Write(hdr); err != nil {
				return nil, err
			}
			if _, err := c.Write(buf[:n]); err != nil {
				return nil, err
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return nil, rerr
		}
	}
	if _, err := c.Write([]byte{0, 0, 0, 0}); err != nil {
		return nil, err
	}
	b, err := io.ReadAll(c)
	if err != nil {
		return nil, err
	}
	resp := strings.TrimRight(string(b), "\x00\n")
	resp = strings.TrimPrefix(resp, "stream: ")
	switch {
	case resp == "OK":
		return &result{Status: "clean", Bytes: total}, nil
	case strings.HasSuffix(resp, " FOUND"):
		return &result{Status: "infected", Signature: strings.TrimSuffix(resp, " FOUND"), Bytes: total}, nil
	default:
		return nil, fmt.Errorf("clamd: %s", resp)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func scanErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errTooLarge):
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": fmt.Sprintf("file exceeds %d bytes", maxBytes)})
	default:
		log.Printf("scan error: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "scan failed: " + err.Error()})
	}
}

func handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		mr, err := r.MultipartReader()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		var results []*result
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			if p.FileName() == "" {
				continue
			}
			res, err := instream(p)
			if err != nil {
				scanErr(w, err)
				return
			}
			res.Filename = p.FileName()
			results = append(results, res)
		}
		if len(results) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no file part found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": results})
		return
	}
	res, err := instream(r.Body)
	if err != nil {
		scanErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func main() {
	port := env("PORT", "8080")
	mux := http.NewServeMux()
	mux.HandleFunc("/scan", handleScan)
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		v, err := command("VERSION")
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"version": v})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		state := "up"
		if p, err := command("PING"); err != nil || p != "PONG" {
			state = "starting"
		}
		writeJSON(w, http.StatusOK, map[string]any{"service": "clamav", "clamd": state, "scan": "POST /scan (multipart file or raw body)", "maxBytes": maxBytes})
	})
	log.Printf("osc-scan-api listening on :%s, clamd at %s", port, clamdAddr)
	srv := &http.Server{Addr: ":" + port, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
