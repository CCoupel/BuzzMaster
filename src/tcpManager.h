#pragma once

#include <AsyncTCP.h>
#include <ArduinoJson.h>
#include <esp_log.h>

const char* TCP_TAG = "TCP";


void startBumperServer()
{
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
    String partial_data=String((char*)data, len);
    ESP_LOGD(TCP_TAG, "Received data from %s: %s", clientID.c_str(), partial_data.c_str());

    clientBuffers[clientID] += partial_data;
    processClientBuffer(clientID, c);
}

void processClientBuffer(const String& clientID, AsyncClient* c) {
    String& jsonBuffer = clientBuffers[clientID];
    int endOfJson;

    while ((endOfJson = jsonBuffer.indexOf('\n')) > 0) {
        String jsonPart = jsonBuffer.substring(0, endOfJson);
        jsonBuffer = jsonBuffer.substring(endOfJson + 1);
        
        parseJSON(jsonPart, c);
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
//    client->onDisconnect(&b_onClientDisconnect, client);
    bumperClients.push_back(client);
    size_t nbClients = bumperClients.size();
    ESP_LOGD(TCP_TAG, "Nb clients : %i", nbClients);

    listClients();
}

static void b_onClientDisconnect(void* arg, AsyncClient* client) {
    String clientID = client->remoteIP().toString();
    ESP_LOGI(TCP_TAG, "Client %s disconnected", clientID.c_str());

    clientBuffers.erase(clientID);
ESP_LOGI(TCP_TAG, "    ClientBuffer erased");
// Supprimer les anciennes connexions de cette IP
    removeClientsByIP(client->remoteIP());    delete client;
ESP_LOGI(TCP_TAG, "    client deleted");

}