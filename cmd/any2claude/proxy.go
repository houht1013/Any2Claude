package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------------------------
//  Stats
// ---------------------------------------------------------------------------

var (
	statRequests  int64
	statErrors    int64
	statStartTime time.Time
)

func GetStats(cm *ConfigManager) map[string]interface{} {
	return map[string]interface{}{
		"requests":    atomic.LoadInt64(&statRequests),
		"errors":      atomic.LoadInt64(&statErrors),
		"uptime":      int(time.Since(statStartTime).Seconds()),
		"model_count": len(cm.BuildModelMap()),
	}
}

// ---------------------------------------------------------------------------
//  Proxy Handler
// ---------------------------------------------------------------------------

type ProxyServer struct {
	configMgr *ConfigManager
	client    *http.Client
}

func NewProxyServer(cm *ConfigManager) *ProxyServer {
	transport := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: false},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true, // pass through as-is for streaming
	}

	return &ProxyServer{
		configMgr: cm,
		client: &http.Client{
			Transport: transport,
			Timeout:   0, // no timeout for streaming
		},
	}
}

func (ps *ProxyServer) isDebug() bool {
	return ps.configMgr.Get().DebugMode
}

func (ps *ProxyServer) debugLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[debug] %s", msg)
	logBuf.Add("DEBUG", msg)
}

func (ps *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS preflight
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.WriteHeader(200)
		return
	}

	atomic.AddInt64(&statRequests, 1)

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", 502)
		atomic.AddInt64(&statErrors, 1)
		return
	}
	defer r.Body.Close()

	if ps.isDebug() {
		ps.debugLog(">> %s %s", r.Method, r.URL.Path)
		if len(body) > 0 && len(body) < 4096 {
			ps.debugLog(">> Body: %s", string(body))
		} else if len(body) >= 4096 {
			ps.debugLog(">> Body: %s ... (%d bytes)", string(body[:512]), len(body))
		}
	}

	// Determine target provider and do model mapping
	var targetProvider *Provider
	isMessages := strings.Contains(r.URL.Path, "/messages")
	streamMode := false
	displayModel := ""

	var anthropicPayload map[string]interface{}

	if isMessages && len(body) > 0 {
		if err := json.Unmarshal(body, &anthropicPayload); err == nil {
			originalModel, _ := anthropicPayload["model"].(string)
			displayModel = originalModel
			modelMap := ps.configMgr.BuildModelMap()

			if resolved, ok := modelMap[originalModel]; ok {
				anthropicPayload["model"] = resolved.RealModel
				targetProvider = resolved.Provider
				log.Printf("[proxy] >> %s -> %s (via %s)", originalModel, resolved.RealModel, resolved.Provider.Name)
				logBuf.Add("INFO", fmt.Sprintf("Model: %s -> %s (via %s)", originalModel, resolved.RealModel, resolved.Provider.Name))
			} else {
				errMsg := fmt.Sprintf("Model \"%s\" is not configured in the model mapping table", originalModel)
				log.Printf("[proxy] ERROR: %s", errMsg)
				logBuf.Add("ERROR", errMsg)
				writeAnthropicError(w, 400, "invalid_request_error", errMsg)
				atomic.AddInt64(&statErrors, 1)
				return
			}

			if s, ok := anthropicPayload["stream"].(bool); ok {
				streamMode = s
			}
		}
	}

	if targetProvider == nil {
		targetProvider = ps.configMgr.GetFirstEnabledProvider()
	}
	if targetProvider == nil {
		http.Error(w, `{"type":"error","error":{"type":"api_error","message":"No enabled provider configured"}}`, 503)
		atomic.AddInt64(&statErrors, 1)
		logBuf.Add("ERROR", "No enabled provider configured")
		return
	}

	// Build upstream URL
	upstream, err := url.Parse(targetProvider.BaseURL)
	if err != nil {
		http.Error(w, `{"type":"error","error":{"type":"api_error","message":"Invalid provider URL"}}`, 502)
		atomic.AddInt64(&statErrors, 1)
		return
	}

	apiFormat := targetProvider.GetAPIFormat()

	// Determine the upstream path
	upstreamPath := strings.TrimRight(upstream.Path, "/")
	requestPath := r.URL.Path

	if apiFormat == "openai" && isMessages {
		// Convert /v1/messages -> /v1/chat/completions for OpenAI format
		requestPath = "/v1/chat/completions"
		if ps.isDebug() {
			ps.debugLog("Rewriting path: %s -> %s (openai format)", r.URL.Path, requestPath)
		}
	}

	targetURL := upstream.Scheme + "://" + upstream.Host + upstreamPath + requestPath
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	// Convert request body if needed
	var upstreamBody []byte
	if apiFormat == "openai" && isMessages && anthropicPayload != nil {
		openaiPayload := convertAnthropicToOpenAIRequest(anthropicPayload)
		upstreamBody, _ = json.Marshal(openaiPayload)
		if ps.isDebug() {
			if len(upstreamBody) < 4096 {
				ps.debugLog(">> OpenAI body: %s", string(upstreamBody))
			} else {
				ps.debugLog(">> OpenAI body: %s ... (%d bytes)", string(upstreamBody[:512]), len(upstreamBody))
			}
		}
	} else {
		// Anthropic format or non-messages endpoint: pass through
		if anthropicPayload != nil {
			upstreamBody, _ = json.Marshal(anthropicPayload)
		} else {
			upstreamBody = body
		}
	}

	if ps.isDebug() {
		ps.debugLog(">> Upstream: %s %s", r.Method, targetURL)
	}

	// Create upstream request
	upReq, err := http.NewRequest(r.Method, targetURL, strings.NewReader(string(upstreamBody)))
	if err != nil {
		http.Error(w, `{"type":"error","error":{"type":"api_error","message":"Failed to create request"}}`, 502)
		atomic.AddInt64(&statErrors, 1)
		return
	}

	// Copy headers (skip hop-by-hop)
	for key, vals := range r.Header {
		low := strings.ToLower(key)
		if low == "host" || low == "connection" || low == "transfer-encoding" || low == "content-length" {
			continue
		}
		for _, v := range vals {
			upReq.Header.Add(key, v)
		}
	}

	// Set auth & content type
	if targetProvider.APIKey != "" {
		if apiFormat == "openai" {
			upReq.Header.Set("Authorization", "Bearer "+targetProvider.APIKey)
			upReq.Header.Del("x-api-key")
		} else {
			upReq.Header.Set("x-api-key", targetProvider.APIKey)
			upReq.Header.Set("Authorization", "Bearer "+targetProvider.APIKey)
		}
	}
	upReq.Header.Set("Content-Type", "application/json")
	upReq.Header.Set("Host", upstream.Host)
	upReq.Header.Set("Content-Length", fmt.Sprintf("%d", len(upstreamBody)))
	// Remove anthropic-specific headers when talking to OpenAI format
	if apiFormat == "openai" {
		upReq.Header.Del("anthropic-version")
		upReq.Header.Del("anthropic-beta")
	}

	// Send request
	resp, err := ps.client.Do(upReq)
	if err != nil {
		errMsg := fmt.Sprintf("Upstream error: %v", err)
		log.Printf("[proxy] ERROR: %s", errMsg)
		logBuf.Add("ERROR", errMsg)
		writeAnthropicError(w, 502, "api_error", errMsg)
		atomic.AddInt64(&statErrors, 1)
		return
	}
	defer resp.Body.Close()

	if ps.isDebug() {
		ps.debugLog("<< Status: %d", resp.StatusCode)
		for k, v := range resp.Header {
			ps.debugLog("<< Header: %s = %s", k, strings.Join(v, ", "))
		}
	}

	// If upstream returned an error, log and forward it (converting format if needed)
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Upstream returned %d: %s", resp.StatusCode, string(respBody))
		log.Printf("[proxy] %s", errMsg)
		logBuf.Add("ERROR", errMsg)
		atomic.AddInt64(&statErrors, 1)

		if apiFormat == "openai" {
			// Convert OpenAI error to Anthropic format
			writeAnthropicError(w, resp.StatusCode, "api_error", fmt.Sprintf("Upstream error (HTTP %d): %s", resp.StatusCode, truncate(string(respBody), 500)))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(resp.StatusCode)
			w.Write(respBody)
		}
		return
	}

	// Handle response based on format
	if apiFormat == "openai" && isMessages {
		if streamMode {
			ps.handleOpenAIStreamToAnthropic(w, resp, displayModel)
		} else {
			ps.handleOpenAINonStreamToAnthropic(w, resp, displayModel)
		}
	} else {
		// Pass-through (anthropic format or non-messages)
		isSSE := false
		for key, vals := range resp.Header {
			low := strings.ToLower(key)
			if low == "transfer-encoding" || low == "connection" {
				continue
			}
			if low == "content-type" {
				for _, v := range vals {
					if strings.Contains(v, "text/event-stream") {
						isSSE = true
					}
				}
			}
			for _, v := range vals {
				w.Header().Add(key, v)
			}
		}
		w.WriteHeader(resp.StatusCode)

		if isSSE || streamMode {
			flusher, ok := w.(http.Flusher)
			buf := make([]byte, 4096)
			for {
				n, err := resp.Body.Read(buf)
				if n > 0 {
					w.Write(buf[:n])
					if ok {
						flusher.Flush()
					}
				}
				if err != nil {
					break
				}
			}
		} else {
			io.Copy(w, resp.Body)
		}
	}
}

// ---------------------------------------------------------------------------
//  Anthropic <-> OpenAI Format Conversion
// ---------------------------------------------------------------------------

func genMsgID() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 24)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return "msg_" + string(b)
}

// convertAnthropicToOpenAIRequest converts Anthropic Messages API request to OpenAI Chat Completions format
func convertAnthropicToOpenAIRequest(payload map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy model and stream
	result["model"] = payload["model"]
	if stream, ok := payload["stream"]; ok {
		result["stream"] = stream
	}
	if maxTokens, ok := payload["max_tokens"]; ok {
		result["max_tokens"] = maxTokens
	}
	if temp, ok := payload["temperature"]; ok {
		result["temperature"] = temp
	}
	if topP, ok := payload["top_p"]; ok {
		result["top_p"] = topP
	}

	// Build messages array
	var messages []map[string]interface{}

	// System message
	if sys, ok := payload["system"]; ok {
		switch s := sys.(type) {
		case string:
			if s != "" {
				messages = append(messages, map[string]interface{}{
					"role":    "system",
					"content": s,
				})
			}
		case []interface{}:
			// Anthropic system can be array of content blocks
			var parts []string
			for _, block := range s {
				if bm, ok := block.(map[string]interface{}); ok {
					if text, ok := bm["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
			if len(parts) > 0 {
				messages = append(messages, map[string]interface{}{
					"role":    "system",
					"content": strings.Join(parts, "\n"),
				})
			}
		}
	}

	// User/assistant messages
	if msgs, ok := payload["messages"].([]interface{}); ok {
		for _, m := range msgs {
			if msg, ok := m.(map[string]interface{}); ok {
				role, _ := msg["role"].(string)
				openaiMsg := map[string]interface{}{
					"role": role,
				}

				switch content := msg["content"].(type) {
				case string:
					openaiMsg["content"] = content
				case []interface{}:
					// Anthropic content blocks -> OpenAI format
					// Check if it's simple text blocks only
					var textParts []string
					hasNonText := false
					for _, block := range content {
						if bm, ok := block.(map[string]interface{}); ok {
							blockType, _ := bm["type"].(string)
							if blockType == "text" {
								if text, ok := bm["text"].(string); ok {
									textParts = append(textParts, text)
								}
							} else if blockType == "tool_use" || blockType == "tool_result" {
								hasNonText = true
							} else {
								hasNonText = true
							}
						}
					}
					if !hasNonText {
						openaiMsg["content"] = strings.Join(textParts, "\n")
					} else {
						// Complex content, pass as-is (some providers support this)
						openaiMsg["content"] = strings.Join(textParts, "\n")
					}
				}

				messages = append(messages, openaiMsg)
			}
		}
	}

	result["messages"] = messages
	return result
}

// handleOpenAINonStreamToAnthropic converts a non-streaming OpenAI response to Anthropic format
func (ps *ProxyServer) handleOpenAINonStreamToAnthropic(w http.ResponseWriter, resp *http.Response, displayModel string) {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeAnthropicError(w, 502, "api_error", "Failed to read upstream response")
		atomic.AddInt64(&statErrors, 1)
		return
	}

	if ps.isDebug() {
		if len(respBody) < 2048 {
			ps.debugLog("<< OpenAI response: %s", string(respBody))
		} else {
			ps.debugLog("<< OpenAI response: %s ... (%d bytes)", string(respBody[:512]), len(respBody))
		}
	}

	var openaiResp map[string]interface{}
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		log.Printf("[proxy] Failed to parse OpenAI response: %v", err)
		logBuf.Add("ERROR", fmt.Sprintf("Failed to parse upstream response: %v | body: %s", err, truncate(string(respBody), 200)))
		writeAnthropicError(w, 502, "api_error", "Failed to parse upstream response")
		atomic.AddInt64(&statErrors, 1)
		return
	}

	// Check for OpenAI error format
	if errObj, ok := openaiResp["error"]; ok {
		errMap, _ := errObj.(map[string]interface{})
		errMsg, _ := errMap["message"].(string)
		writeAnthropicError(w, 400, "api_error", errMsg)
		atomic.AddInt64(&statErrors, 1)
		return
	}

	// Extract content from OpenAI response
	content := ""
	finishReason := "end_turn"
	inputTokens := 0
	outputTokens := 0

	if choices, ok := openaiResp["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				content, _ = message["content"].(string)
			}
			if fr, ok := choice["finish_reason"].(string); ok {
				finishReason = mapFinishReason(fr)
			}
		}
	}

	if usage, ok := openaiResp["usage"].(map[string]interface{}); ok {
		if pt, ok := usage["prompt_tokens"].(float64); ok {
			inputTokens = int(pt)
		}
		if ct, ok := usage["completion_tokens"].(float64); ok {
			outputTokens = int(ct)
		}
	}

	// Build Anthropic response
	anthropicResp := map[string]interface{}{
		"id":   genMsgID(),
		"type": "message",
		"role": "assistant",
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": content,
			},
		},
		"model":       displayModel,
		"stop_reason": finishReason,
		"stop_sequence": nil,
		"usage": map[string]interface{}{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
	}

	respJSON, _ := json.Marshal(anthropicResp)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(respJSON)

	logBuf.Add("INFO", fmt.Sprintf("Response: %d tokens in, %d tokens out", inputTokens, outputTokens))
}

// handleOpenAIStreamToAnthropic converts streaming OpenAI SSE to Anthropic SSE format
func (ps *ProxyServer) handleOpenAIStreamToAnthropic(w http.ResponseWriter, resp *http.Response, displayModel string) {
	flusher, hasFlusher := w.(http.Flusher)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(200)

	msgID := genMsgID()

	// Send message_start
	messageStart := map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":      msgID,
			"type":    "message",
			"role":    "assistant",
			"content": []interface{}{},
			"model":   displayModel,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]interface{}{
				"input_tokens":  0,
				"output_tokens": 0,
			},
		},
	}
	writeSSEEvent(w, "message_start", messageStart)
	if hasFlusher {
		flusher.Flush()
	}

	// Send content_block_start
	blockStart := map[string]interface{}{
		"type":  "content_block_start",
		"index": 0,
		"content_block": map[string]interface{}{
			"type": "text",
			"text": "",
		},
	}
	writeSSEEvent(w, "content_block_start", blockStart)
	if hasFlusher {
		flusher.Flush()
	}

	// Send ping
	writeSSEEvent(w, "ping", map[string]interface{}{"type": "ping"})
	if hasFlusher {
		flusher.Flush()
	}

	// Read OpenAI SSE stream and convert each chunk
	scanner := bufio.NewScanner(resp.Body)
	// Increase scanner buffer for large chunks
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	outputTokens := 0
	finishReason := "end_turn"

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		data = strings.TrimSpace(data)

		if data == "[DONE]" {
			if ps.isDebug() {
				ps.debugLog("<< SSE: [DONE]")
			}
			break
		}

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			if ps.isDebug() {
				ps.debugLog("<< SSE parse error: %v | data: %s", err, truncate(data, 200))
			}
			continue
		}

		// Extract content delta from OpenAI chunk
		choices, ok := chunk["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			continue
		}
		choice, ok := choices[0].(map[string]interface{})
		if !ok {
			continue
		}

		// Check finish_reason
		if fr, ok := choice["finish_reason"].(string); ok && fr != "" {
			finishReason = mapFinishReason(fr)
			continue
		}

		delta, ok := choice["delta"].(map[string]interface{})
		if !ok {
			continue
		}

		contentStr, ok := delta["content"].(string)
		if !ok || contentStr == "" {
			continue
		}

		outputTokens++

		// Send content_block_delta
		blockDelta := map[string]interface{}{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": contentStr,
			},
		}
		writeSSEEvent(w, "content_block_delta", blockDelta)
		if hasFlusher {
			flusher.Flush()
		}
	}

	// Send content_block_stop
	writeSSEEvent(w, "content_block_stop", map[string]interface{}{
		"type":  "content_block_stop",
		"index": 0,
	})
	if hasFlusher {
		flusher.Flush()
	}

	// Send message_delta
	writeSSEEvent(w, "message_delta", map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason":   finishReason,
			"stop_sequence": nil,
		},
		"usage": map[string]interface{}{
			"output_tokens": outputTokens,
		},
	})
	if hasFlusher {
		flusher.Flush()
	}

	// Send message_stop
	writeSSEEvent(w, "message_stop", map[string]interface{}{
		"type": "message_stop",
	})
	if hasFlusher {
		flusher.Flush()
	}

	logBuf.Add("INFO", fmt.Sprintf("Stream complete: ~%d chunks", outputTokens))
}

// writeSSEEvent writes a single SSE event in Anthropic format
func writeSSEEvent(w http.ResponseWriter, event string, data interface{}) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(jsonData))
}

// writeAnthropicError writes an Anthropic-formatted error response
func writeAnthropicError(w http.ResponseWriter, status int, errType string, message string) {
	resp := map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    errType,
			"message": message,
		},
	}
	respJSON, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(respJSON)
}

// mapFinishReason maps OpenAI finish_reason to Anthropic stop_reason
func mapFinishReason(openaiReason string) string {
	switch openaiReason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "content_filter":
		return "end_turn"
	default:
		return "end_turn"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
