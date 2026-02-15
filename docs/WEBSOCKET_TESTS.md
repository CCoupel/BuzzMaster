# WebSocket Buzzer Test Plan

## Overview

This document describes the test plan for the WebSocket buzzer feature (v3.0.0). BuzzClick buzzers can now connect via WebSocket instead of TCP, enabling browser-based communication while maintaining backward compatibility with existing TCP buzzers.

## Test Categories

### 1. Unit Tests (Automated)

#### 1.1 Server WebSocket Hub Tests

**File**: `server-go/internal/server/websocket_buzzer_test.go`

| Test | Description | Status |
|------|-------------|--------|
| `TestWebSocketHub_ClientConnect` | Single client connects via WebSocket | Written |
| `TestWebSocketHub_ClientDisconnect` | Client disconnects, hub count decreases | Written |
| `TestWebSocketHub_MultipleClients` | Multiple simultaneous WebSocket connections | Written |
| `TestWebSocketHub_ReceiveHelloMessage` | Parse HELLO message with MAC and version | Written |
| `TestWebSocketHub_ReceiveButtonMessage` | Parse BUTTON message with press time | Written |
| `TestWebSocketHub_ReceivePongMessage` | Parse PONG ready response | Written |
| `TestWebSocketHub_ReceiveMultipleMessages` | 3 rapid messages received in order | Written |
| `TestWebSocketHub_SendToClient` | Server sends PING to specific client | Written |
| `TestWebSocketHub_Broadcast` | Server broadcasts UPDATE to all clients | Written |
| `TestWebSocketHub_BroadcastRaw` | Raw byte broadcast | Written |
| `TestWebSocketHub_SetClientType` | Client type assignment (admin/TV/vplayer) | Written |
| `TestWebSocketHub_GetClientCounts` | Count clients by type | Written |
| `TestWebSocketHub_OnClientChangeCallback` | Callback fires on connect/disconnect | Written |
| `TestWebSocketHub_OnMessageCallback` | Callback fires on incoming message | Written |
| `TestWebSocketHub_BuzzerIdentificationByMAC` | MAC address preserved in HELLO | Written |
| `TestWebSocketHub_IncomingMessageSource` | Source field is "WebSocket" | Written |
| `TestWebSocketHub_ConcurrentConnections` | 10 simultaneous connections | Written |
| `TestWebSocketHub_ConcurrentBroadcast` | 50 concurrent broadcasts | Written |
| `TestWebSocketHub_IncomingChannelFull` | No deadlock when channel at capacity | Written |
| `TestWebSocketHub_SendChannelFull_ClientRemoved` | Client removed when send buffer full | Written |
| `TestMessage_SerializeForWebSocket_NoNullTerminator` | WS messages have no \0 | Written |
| `TestMessage_SerializeForTCP_HasNullTerminator` | TCP messages have \0 | Written |
| `TestParseSingle_ValidJSON` | Parse various valid JSON formats | Written |
| `TestParseSingle_InvalidJSON` | Reject empty, broken, incomplete JSON | Written |

#### 1.2 Engine WebSocket Integration Tests

**File**: `server-go/internal/game/engine_websocket_test.go`

| Test | Description | Status |
|------|-------------|--------|
| `TestEngine_WebSocket_BuzzerRegistration` | Register WS buzzer with MAC | Written |
| `TestEngine_WebSocket_MultipleBuzzerRegistration` | Register multiple WS buzzers | Written |
| `TestEngine_WebSocket_BuzzerReady` | WS buzzer responds PONG, team marked ready | Written |
| `TestEngine_WebSocket_ButtonPress` | WS buzzer press records time/button/status | Written |
| `TestEngine_WebSocket_QCMButtonPress` | WS buzzer QCM press maps to AnswerColor | Written |
| `TestEngine_HybridMode_TCPAndWebSocketBuzzers` | TCP + WS buzzers in different teams | Written |
| `TestEngine_HybridMode_SameTeamMixedTransport` | TCP + WS buzzers in same team, one press per team | Written |
| `TestEngine_WebSocket_VPlayerBuzzerCoexistence` | WS physical buzzer + VPlayer QCM invalidation | Written |
| `TestEngine_WebSocket_ScoreTracking` | Score awards to WS buzzer | Written |
| `TestEngine_WebSocket_StateResetOnReady` | Ready resets WS buzzer state | Written |
| `TestEngine_WebSocket_BuzzerPressCallback` | OnBuzzerPress callback for WS buzzer | Written |
| `TestEngine_WebSocket_ConcurrentPresses` | 4 WS buzzers press simultaneously (-race) | Written |
| `TestEngine_WebSocket_PressTimePreserved` | Microsecond press time preserved exactly | Written |
| `TestEngine_WebSocket_QCMHintsAtBuzz` | HintsAtBuzz recorded for WS buzzer | Written |
| `TestEngine_WebSocket_RAZScores` | RAZ resets WS buzzer scores | Written |
| `TestEngine_WebSocket_ForceReady` | ForceReady marks WS buzzers as ready | Written |

### 2. Manual Firmware Tests

These tests require physical ESP32-C3 BuzzClick hardware with WebSocket firmware.

#### 2.1 Connection Tests

| # | Test | Steps | Expected Result |
|---|------|-------|-----------------|
| F1 | WebSocket connection | Power on buzzer, verify WS connection to server | LED turns green, server logs "WebSocket client connected" |
| F2 | Reconnection after WiFi drop | Disconnect WiFi for 5s, reconnect | Buzzer reconnects within 10s, LED green |
| F3 | Reconnection after server restart | Restart server while buzzer connected | Buzzer reconnects within 10s |
| F4 | Connection during game | Connect buzzer while game is STARTED | Buzzer appears in bumper list, cannot buzz until next round |

#### 2.2 Latency Tests

| # | Test | Steps | Expected Result |
|---|------|-------|-----------------|
| L1 | BUZZ latency < 50ms | Press buzzer during game, measure RTT | Time between physical press and server receipt < 50ms |
| L2 | LED command latency | Send LED command from server | LED changes within 100ms |
| L3 | PONG latency | Server sends PING, measure PONG time | PONG received within 200ms |

#### 2.3 LED Tests

| # | Test | Steps | Expected Result |
|---|------|-------|-----------------|
| LED1 | Connection indicator | Connect buzzer | LED green (connected) |
| LED2 | USB config mode | Connect via USB, enter AT mode | LED orange/magenta |
| LED3 | Factory reset | Hold button 3s at boot | LED blue blink, then magenta |
| LED4 | Game state LEDs | Play a round | LEDs follow game state (prepare/start/stop) |

#### 2.4 Memory Tests

| # | Test | Steps | Expected Result |
|---|------|-------|-----------------|
| M1 | Heap usage | Monitor heap via serial after 1h | Heap usage stable, no growth |
| M2 | WebSocket buffer | Send 100 rapid messages | No buffer overflow, all processed |

### 3. Load Tests

These tests verify system stability under sustained load.

| # | Test | Setup | Duration | Pass Criteria |
|---|------|-------|----------|---------------|
| C1 | 10 WS buzzers | 10 simulated WS clients | 10 min | All messages received, no disconnects |
| C2 | Latency under load | 10 WS buzzers, rapid presses | 5 min | Avg latency < 50ms, max < 200ms |
| C3 | Long duration | 4 WS buzzers | 1 hour | No memory leak, no disconnects, stable response time |
| C4 | Rapid reconnection | 1 buzzer reconnecting every 5s | 10 min | Server handles all reconnects, no goroutine leak |

### 4. Compatibility Tests (TCP + WebSocket Hybrid)

These tests verify backward compatibility with existing TCP buzzers.

| # | Test | Setup | Expected Result |
|---|------|-------|-----------------|
| H1 | TCP buzzer still works | Connect old firmware buzzer (1.209.3) via TCP | Buzzer works normally, no regression |
| H2 | Mixed TCP + WS game | 2 TCP buzzers + 2 WS buzzers | All buzzers participate, correct scoring |
| H3 | TCP PING/PONG | TCP buzzer receives PING, responds PONG | PONG processed, team marked ready |
| H4 | WS PING/PONG | WS buzzer receives PING, responds PONG | PONG processed, team marked ready |
| H5 | Mixed broadcast | Server sends UPDATE to all | Both TCP and WS clients receive |
| H6 | Migration test | Replace TCP buzzer with WS buzzer mid-session | New buzzer registers, old bumper replaced |

## Running Tests

### Automated Unit Tests

```bash
# All tests
cd server-go
go test ./... -v -cover

# WebSocket server tests only
go test ./internal/server/ -run TestWebSocket -v

# Engine WebSocket tests only
go test ./internal/game/ -run TestEngine_WebSocket -v

# Hybrid mode tests
go test ./internal/game/ -run TestEngine_HybridMode -v

# Race condition detection
go test ./... -race -v
```

### Coverage Report

```bash
cd server-go
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Test Environment

| Component | Version |
|-----------|---------|
| Go | 1.21+ |
| gorilla/websocket | v1.5+ |
| ESP32-C3 SDK | ESP-IDF 5.x |
| Chrome (Web Serial) | 89+ |
| Test OS | Windows 10/11, Raspberry Pi OS |
