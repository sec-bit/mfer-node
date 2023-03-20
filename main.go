package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type jsonRPCRequest struct {
	Jsonrpc string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      int             `json:"id"`
}

var (
	targetURL   string
	methodsList string
	methods     map[string]bool
	listenAddr  string
	listenPort  int
)

func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	return value
}

func getIntEnv(key string, fallback int) int {
	valueStr, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return fallback
	}
	return value
}

func main() {
	targetURL = getEnv("TARGET", "http://mfer-node:10545")
	methodsList = getEnv("METHODS", "mfer_traceTransactionBundle")
	listenAddr = getEnv("LISTEN_ADDR", "0.0.0.0")
	listenPort = getIntEnv("LISTEN_PORT", 10545)

	if targetURL == "" || methodsList == "" {
		log.Fatal("Both target and methods arguments are required")
	}

	methods = make(map[string]bool)
	for _, method := range strings.Split(methodsList, ",") {
		methods[method] = true
	}

	http.HandleFunc("/", handleRequest)
	addr := fmt.Sprintf("%s:%d", listenAddr, listenPort)
	log.Printf("Listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	var request jsonRPCRequest
	err = json.Unmarshal(body, &request)
	if err != nil {
		http.Error(w, "Error parsing JSON-RPC request", http.StatusBadRequest)
		return
	}

	log.Printf("User request: method=%s, id=%d\n", request.Method, request.ID)
	if methods[request.Method] {
		client := &http.Client{}
		bodyReader := strings.NewReader(string(body))
		targetReq, err := http.NewRequest(r.Method, targetURL, bodyReader)
		if err != nil {
			http.Error(w, "Error creating target request", http.StatusInternalServerError)
			return
		}

		targetReq.Header = r.Header
		resp, err := client.Do(targetReq)
		if err != nil {
			log.Println(err)
			http.Error(w, "Error forwarding request", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "Error reading response body", http.StatusInternalServerError)
			return
		}

		log.Printf("Server response: status=%d, id=%d\n", resp.StatusCode, request.ID)

		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
	} else {
		http.Error(w, fmt.Sprintf("Method not allowed, only %s accepted", methodsList), http.StatusMethodNotAllowed)
		return
	}
}
