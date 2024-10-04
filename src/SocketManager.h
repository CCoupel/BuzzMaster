#include <ArduinoJson.h>
#include <esp_log.h>

const char* SOCKET_TAG = "SOCKET";

void handleWebSocketData(AsyncWebSocketClient *client, uint8_t *data, size_t len) {
  ESP_LOGI(SOCKET_TAG, "Received WebSocket data from client %u", client->id());
  
  JsonDocument receivedData;
  DeserializationError error = deserializeJson(receivedData, data, len);
  if (error) {
      ESP_LOGE(SOCKET_TAG, "Failed to parse JSON: %s", error.c_str());
      return;
  }

  if (!receivedData.containsKey("ACTION") || !receivedData.containsKey("MSG")) {
      ESP_LOGE(SOCKET_TAG, "Invalid JSON structure");
      return;
  }

  const char* action = receivedData["ACTION"];
  JsonObject message = receivedData["MSG"];

  parseDataFromSocket(action, message);
}

void parseDataFromSocket(const char* action, JsonObject& message) {
  ESP_LOGI(SOCKET_TAG, "Parsing action: %s", action);
  String output;
  serializeJson(message, output);
  ESP_LOGI(SOCKET_TAG, "Parsing message: %s", output.c_str());
  switch (hash(action)) {
    case hash("PING"):
      // Handle ping
      break;
    case hash("HELLO"):
      notifyAll();
      break;
    case hash("FULL"):
      setBumpers(message["bumpers"]);
      setTeams(message["teams"]);
      notifyAll();
      break;
    case hash("UPDATE"):
      updateTeams(message["teams"]);
      updateBumpers(message["bumpers"]);
      break;
    case hash("DELETE"):
      //update("DELETE", message);
      break;
    case hash("RESET"):
      resetServer();
      break;
    case hash("REBOOT"):
      rebootServer();
      break;
    case hash("START"):
      startGame();
      break;
    case hash("STOP"):
      stopGame();
      break;
    case hash("PAUSE"):
      pauseAllGame();
      break;
    case hash("CONTINUE"):
      continueGame();
      break;
    default:
      ESP_LOGW(SOCKET_TAG, "Unrecognized action: %s", action);
      break;
  }
}

void onWsEvent(AsyncWebSocket *server, AsyncWebSocketClient *client, AwsEventType type, void *arg, uint8_t *data, size_t len) {
  String output;
  JsonDocument receivedData;
  switch(type) {
    case WS_EVT_CONNECT:
      // Quand un client se connecte, envoyer un message
      ESP_LOGI(SOCKET_TAG, "Client %u connecté\n", client->id());
      //client->text("Bienvenue sur le serveur WebSocket !");
      break;
    case WS_EVT_DISCONNECT:
      // Quand un client se déconnecte
      ESP_LOGI(SOCKET_TAG, "Client %u déconnecté\n", client->id());
      break;
    case WS_EVT_DATA:
        handleWebSocketData(client, data, len);
        break;
    default:
        break;
  }
}
