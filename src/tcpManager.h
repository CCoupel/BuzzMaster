#include <AsyncTCP.h>
#include <ArduinoJson.h>
#include <esp_log.h>

const char* TCP_TAG = "TCP";
unsigned int CONTROLER_PORT = 1234;  // Port d'écoute local

void startBumperServer()
{
  bumperServer = new AsyncServer(CONTROLER_PORT);
  bumperServer->onClient(&b_onCLientConnect, bumperServer);
  
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

static void b_onClientDisconnect(void* arg, AsyncClient* client) {
    String clientID = client->remoteIP().toString();
    ESP_LOGI(TCP_TAG, "Client %s disconnected", clientID.c_str());

    clientBuffers.erase(clientID);
ESP_LOGI(TCP_TAG, "    ClientBuffer erased");
    bumperClients.erase(std::remove(bumperClients.begin(), bumperClients.end(), client), bumperClients.end());
ESP_LOGI(TCP_TAG, "    bumperClients erased");
    delete client;
ESP_LOGI(TCP_TAG, "    client deleted");

}

static void b_onCLientConnect(void* arg, AsyncClient* client) {
    ESP_LOGI(TCP_TAG, "New client connected: %s", client->remoteIP().toString().c_str());
    client->onData(&b_handleData, NULL);
    client->onDisconnect(&b_onClientDisconnect, NULL);
    bumperClients.push_back(client);
}
