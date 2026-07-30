package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/koron/go-ssdp"
	"github.com/rs/cors"
)

// Configuration Constants
const (
	TVConnectionTimeout = 10 * time.Second
	HealthCheckInterval = 10 * time.Second
	SSDPScanDelay       = 5 * time.Second
)

var (
	targetTVIP    string
	ipMutex       sync.RWMutex
	isTVConnected bool
)

//go:embed static/*
var staticFiles embed.FS

type CommandRequest struct {
	KeyEvent string `json:"keyEvent"`
}

func main() {
	// Start background TV discovery & health-check loop
	go tvDiscoveryLoop()

	mux := http.NewServeMux()

	// API: Get TV connection status and IP
	mux.HandleFunc("/api/discover", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ipMutex.RLock()
		defer ipMutex.RUnlock()
		fmt.Fprintf(w, `{"ip": "%s", "connected": %t}`, targetTVIP, isTVConnected)
	})

	// API: Send remote control command
	mux.HandleFunc("/api/remote", handleProxyCommand)

	// Serve Embedded PWA Frontend from /static
	subFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to create sub filesystem: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(subFS)))

	// Enable CORS
	handler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"POST", "GET", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type"},
	}).Handler(mux)

	fmt.Println("🚀 Panasonic Go Remote Proxy running on port 3000...")
	log.Fatal(http.ListenAndServe(":3000", handler))
}

func tvDiscoveryLoop() {
	isFirstDiscovery := true
	wasConnected := false
	hasAnnouncedColdStart := false

	for {
		ipMutex.RLock()
		currentIP := targetTVIP
		ipMutex.RUnlock()

		if currentIP != "" && checkTVHealth(currentIP) {
			ipMutex.Lock()
			isTVConnected = true
			ipMutex.Unlock()
			wasConnected = true
			time.Sleep(HealthCheckInterval)
			continue
		}

		ipMutex.Lock()
		isTVConnected = false
		ipMutex.Unlock()

		if isFirstDiscovery {
			if !hasAnnouncedColdStart {
				fmt.Println("🔍 Scanning local network for Panasonic TV (Cold Start)...")
				hasAnnouncedColdStart = true
			}
		} else if wasConnected {
			fmt.Println("🔍 TV offline. Scanning local network for reconnection...")
			wasConnected = false
		}

		foundIP, err := discoverPanasonicTV()
		if err == nil && checkTVHealth(foundIP) {
			ipMutex.Lock()
			targetTVIP = foundIP
			isTVConnected = true
			ipMutex.Unlock()

			if isFirstDiscovery {
				fmt.Printf("✅ Discovered Panasonic TV at: %s\n", foundIP)
				isFirstDiscovery = false
			} else {
				fmt.Printf("✅ Reconnected to Panasonic TV at: %s\n", foundIP)
			}
			time.Sleep(HealthCheckInterval)
		} else {
			time.Sleep(SSDPScanDelay)
		}
	}
}

// checkTVHealth tests if the TV's control service is actually responsive
func checkTVHealth(ip string) bool {
	tvURL := fmt.Sprintf("http://%s:55000/nrc/control_0", ip)

	client := &http.Client{
		Timeout: TVConnectionTimeout,
	}

	resp, err := client.Get(tvURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return true
}

// discoverPanasonicTV listens for UPnP/SSDP responses looking for Panasonic media servers or renderers
func discoverPanasonicTV() (string, error) {
	searchTarget := "urn:schemas-upnp-org:device:MediaRenderer:1"
	list, err := ssdp.Search(searchTarget, 3, "")
	if err != nil {
		return "", err
	}

	for _, resp := range list {
		if strings.Contains(strings.ToLower(resp.Server), "panasonic") || strings.Contains(strings.ToLower(resp.USN), "panasonic") {
			u, err := url.Parse(resp.Location)
			if err == nil {
				host, _, _ := net.SplitHostPort(u.Host)
				if host != "" {
					return host, nil
				}
			}
		}
	}

	listAll, err := ssdp.Search(ssdp.All, 3, "")
	if err == nil {
		for _, resp := range listAll {
			if strings.Contains(strings.ToLower(resp.Server), "panasonic") {
				u, err := url.Parse(resp.Location)
				if err == nil {
					host, _, _ := net.SplitHostPort(u.Host)
					if host != "" {
						return host, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("panasonic tv not found via ssdp")
}

// handleProxyCommand translates JSON web requests into SOAP envelopes for the CX802B
func handleProxyCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ipMutex.RLock()
	currentIP := targetTVIP
	connected := isTVConnected
	ipMutex.RUnlock()

	if !connected || currentIP == "" {
		http.Error(w, "TV is currently offline or sleeping", http.StatusServiceUnavailable)
		return
	}

	// Safely decode incoming JSON payload
	var reqBody CommandRequest
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil || reqBody.KeyEvent == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	soapXML := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:X_SendKey xmlns:u="urn:panasonic-com:service:p00NetworkControl:1">
      <X_KeyEvent>%s</X_KeyEvent>
    </u:X_SendKey>
  </s:Body>
</s:Envelope>`, reqBody.KeyEvent)

	tvURL := fmt.Sprintf("http://%s:55000/nrc/control_0", currentIP)

	proxyReq, _ := http.NewRequest("POST", tvURL, strings.NewReader(soapXML))
	proxyReq.Header.Set("Content-Type", "text/xml; charset=\"utf-8\"")
	proxyReq.Header.Set("SOAPACTION", `"urn:panasonic-com:service:p00NetworkControl:1#X_SendKey"`)

	client := &http.Client{Timeout: TVConnectionTimeout}
	resp, err := client.Do(proxyReq)
	if err != nil {
		ipMutex.Lock()
		isTVConnected = false
		ipMutex.Unlock()

		http.Error(w, fmt.Sprintf("TV communication error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func extractKey(input string) string {
	cleaned := strings.Trim(input, `"{}/`)
	if strings.HasPrefix(cleaned, "keyEvent:") {
		parts := strings.Split(cleaned, ":")
		if len(parts) > 1 {
			return strings.Trim(parts[1], `" `)
		}
	}
	return cleaned
}
