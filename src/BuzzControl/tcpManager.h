#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"
#include "messages_received.h"

#include <AsyncTCP.h>
#include <ArduinoJson.h>

const char* TCP_TAG = "TCP";

void startBumperServer() {
  bumperServer = new AsyncServer(CONTROLER_PORT);
  bumperServer->onClient(&b_onCLientConnect, bumperServer);
  
  // Afficher les adresses IP des interfaces
  IPAddress staIP = WiFi.localIP();
  IPAddress apIP = WiFi.softAPIP();
  
  ESP_LOGI(TCP_TAG, "STA IP Address: %s", staIP.toString().c_str());
  ESP_LOGI(TCP_TAG, "AP IP Address: %s", apIP.toString().c_str());

  bumperServer->begin();
  ESP_LOGI(TCP_TAG, "BUMPER server started on port %i", CONTROLER_PORT);
}

void b_handleData(void* arg, AsyncClient* c, void *data, size_t len) {
    String clientID = c->remoteIP().toString();
    String partial_data = String((char*)data, len);
    ESP_LOGD(TCP_TAG, "Received data from %s: %s", clientID.c_str(), partial_data.c_str());

    clientBuffers[clientID] += partial_data;
    processClientBuffer(clientID, c);
}

void processClientBuffer(const String& clientID, AsyncClient* c) {
    String& jsonBuffer = clientBuffers[clientID];
    int endOfJson;
    ESP_LOGD(TCP_TAG, "processClientBuffer %s", jsonBuffer.c_str());

    while ((endOfJson = jsonBuffer.indexOf('\0')) > 0) {
        String jsonPart = jsonBuffer.substring(0, endOfJson);
        jsonBuffer = jsonBuffer.substring(endOfJson + 1);
        
        // Au lieu de traiter directement, envoyez à la file d'attente des messages entrants
        enqueueIncomingMessage("TCP", jsonPart.c_str(), c);
    }
}

static void listClients() {
    Serial.printf("#####");
    for(AsyncClient* client : bumperClients) {
        Serial.printf("Client IP: %s\n", client->remoteIP().toString().c_str());
    }
}

// Version qui supprime tous les clients d'une IP donnée
static void removeClientsByIP(const IPAddress& ip) {
    ESP_LOGI(TCP_TAG, "Removing all clients with IP: %s", ip.toString().c_str());
    
    for(auto it = bumperClients.begin(); it != bumperClients.end();) {
        AsyncClient* existingClient = *it;
        if(existingClient->remoteIP() == ip) {
            ESP_LOGI(TCP_TAG, "Removing old connection from IP: %s", ip.toString().c_str());
            existingClient->close(true);
            delete existingClient;
            it = bumperClients.erase(it);
        } else {
            ++it;
        }
    }
}

static void b_onCLientConnect(void* arg, AsyncClient* client) {
    ESP_LOGI(TCP_TAG, "New client connected: %s", client->remoteIP().toString().c_str());
    listClients();
    // Supprimer les anciennes connexions de cette IP
    removeClientsByIP(client->remoteIP());

    client->onData(&b_handleData, NULL);
    bumperClients.push_back(client);
    size_t nbClients = bumperClients.size();
    ESP_LOGD(TCP_TAG, "Nb clients : %i", nbClients);

    listClients();
}

static void b_onClientDisconnect(void* arg, AsyncClient* client) {
    ESP_LOGI(TCP_TAG, "Client disconnected: %s", client->remoteIP().toString().c_str());
    
    // Rechercher et supprimer le client de la liste
    for (auto it = bumperClients.begin(); it != bumperClients.end(); ++it) {
        if (*it == client) {
            bumperClients.erase(it);
            break;
        }
    }
}