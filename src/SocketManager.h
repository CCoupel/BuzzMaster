#include <ArduinoJson.h>
#include <esp_log.h>

const char* SOCKET_TAG = "SOCKET";

void parseDataFromSocket(const char* action, const JsonObject& message) {
  ESP_LOGI(SOCKET_TAG, "Parsing action: %s", action);
  String output="";
  serializeJson(message, output);

  if (output.length() > 0) {
    ESP_LOGI(SOCKET_TAG, "Parsing message: %i: %s", output.length(), output.c_str());
  } else {
    output="!! NULL !!";
    ESP_LOGI(SOCKET_TAG, "Parsing null message: %s", output.c_str());
  }
  

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

void handleWebSocketData(AsyncWebSocketClient *client, uint8_t *data, size_t len) {
  ESP_LOGI(SOCKET_TAG, "Received WebSocket data from client %u", client->id());
  if (len > 0) {
    // Limiter la taille des données loguées pour éviter de surcharger les logs
    const size_t maxLogLength = 100;
    size_t logLength = len < maxLogLength ? len : maxLogLength;

    // Créer une copie temporaire des données pour s'assurer qu'elles sont null-terminées
    char logBuffer[maxLogLength + 1];
    memcpy(logBuffer, data, logLength);
    logBuffer[logLength] = '\0';  // Ajouter un caractère nul à la fin

    // Logger les données en tant que chaîne de caractères
    ESP_LOGD(SOCKET_TAG, "Received WebSocket data (first %u bytes): %s%s", 
             logLength, logBuffer, len > maxLogLength ? "..." : "");
  } else {
    ESP_LOGD(SOCKET_TAG, "Received empty WebSocket data");
  }

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
  JsonObject message = receivedData["MSG"].as<JsonObject>();

  parseDataFromSocket(action, message);
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
