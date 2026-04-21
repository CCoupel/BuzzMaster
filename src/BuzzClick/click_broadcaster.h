#pragma once

#include <WiFi.h>
#include <AsyncUDP.h>
#include "esp_log.h"

static const char* BCAST_TAG = "Broadcaster";

// Maximum number of server IPs to store from broadcast
const int MAX_SERVER_IPS = 8;

// Broadcast protocol prefix
const char* BCAST_PREFIX = "BUZZ_SERVER";

// Server IPs discovered via UDP broadcast (RAM only, no NVS)
struct BroadcastDiscovery {
    String ips[MAX_SERVER_IPS];
    int ipCount;
    uint16_t serverPort;
    bool received;              // true after first valid heartbeat
    unsigned long lastReceived; // millis() of last heartbeat
};

static BroadcastDiscovery discovery = {{}, 0, 80, false, 0};
static AsyncUDP broadcastUdp;

// Parse BUZZ_SERVER|IP1|IP2|...|PORT format
// Returns true if parsing succeeded
// sourceIp is the IP address of the UDP packet sender (for logging)
bool parseBroadcastHeartbeat(const char* data, size_t len, const IPAddress& sourceIp) {
    // Remove trailing null if present
    String msg;
    if (len > 0 && data[len - 1] == '\0') {
        msg = String(data, len - 1);
    } else {
        msg = String(data, len);
    }

    // Must start with BUZZ_SERVER|
    if (!msg.startsWith("BUZZ_SERVER|")) {
        ESP_LOGD(BCAST_TAG, "Not a BUZZ_SERVER message: %s", msg.c_str());
        return false;
    }

    // Split by '|': BUZZ_SERVER | IP1 | IP2 | ... | PORT
    // Minimum: BUZZ_SERVER|PORT (no IPs) or BUZZ_SERVER|IP|PORT
    int partCount = 0;
    String parts[MAX_SERVER_IPS + 2]; // prefix + up to MAX_SERVER_IPS IPs + port

    int startIdx = 0;
    for (int i = 0; i <= (int)msg.length(); i++) {
        if (i == (int)msg.length() || msg.charAt(i) == '|') {
            if (partCount < MAX_SERVER_IPS + 2) {
                parts[partCount] = msg.substring(startIdx, i);
                partCount++;
            }
            startIdx = i + 1;
        }
    }

    // Need at least prefix + port = 2 parts
    if (partCount < 2) {
        ESP_LOGW(BCAST_TAG, "Invalid broadcast format (too few parts): %s", msg.c_str());
        return false;
    }

    // Last part is PORT
    uint16_t port = (uint16_t)parts[partCount - 1].toInt();
    if (port == 0) {
        ESP_LOGW(BCAST_TAG, "Invalid port in broadcast: %s", parts[partCount - 1].c_str());
        return false;
    }

    // Middle parts (index 1 to partCount-2) are IPs
    int newIpCount = partCount - 2; // exclude prefix and port

    // Update discovery state
    discovery.serverPort = port;
    discovery.ipCount = 0;

    for (int i = 0; i < newIpCount && i < MAX_SERVER_IPS; i++) {
        discovery.ips[i] = parts[i + 1]; // skip prefix at index 0
        discovery.ipCount++;
    }

    discovery.received = true;
    discovery.lastReceived = millis();

    ESP_LOGI(BCAST_TAG, "Heartbeat received from %s — %d IPs, port=%d",
             sourceIp.toString().c_str(), discovery.ipCount, discovery.serverPort);
    for (int i = 0; i < discovery.ipCount; i++) {
        ESP_LOGI(BCAST_TAG, "  IP[%d]: %s", i, discovery.ips[i].c_str());
    }

    return true;
}

// Start listening for UDP broadcast heartbeats on the given port
bool startBroadcastListener(uint16_t port) {
    if (!broadcastUdp.listen(port)) {
        ESP_LOGE(BCAST_TAG, "Failed to start UDP listener on port %d", port);
        return false;
    }

    broadcastUdp.onPacket([](AsyncUDPPacket packet) {
        if (packet.length() <= 0) return;

        const char* data = (const char*)packet.data();
        size_t len = packet.length();

        // Only process BUZZ_SERVER messages (ignore other UDP traffic on this port)
        if (len >= 12 && strncmp(data, "BUZZ_SERVER", 11) == 0) {
            parseBroadcastHeartbeat(data, len, packet.remoteIP());
        }
    });

    ESP_LOGI(BCAST_TAG, "Listening on %s:%d",
             WiFi.localIP().toString().c_str(), port);
    return true;
}

// Stop the broadcast listener
void stopBroadcastListener() {
    broadcastUdp.close();
    ESP_LOGI(BCAST_TAG, "UDP broadcast listener stopped");
}

// Check if at least one broadcast heartbeat has been received
bool hasBroadcastDiscovery() {
    return discovery.received && discovery.ipCount > 0;
}

// Get the discovery result (caller should check hasBroadcastDiscovery() first)
const BroadcastDiscovery& getBroadcastDiscovery() {
    return discovery;
}

// Reset discovery state (e.g., before waiting for new broadcast)
void resetBroadcastDiscovery() {
    discovery.ipCount = 0;
    discovery.received = false;
    discovery.lastReceived = 0;
    discovery.serverPort = 80;
}
