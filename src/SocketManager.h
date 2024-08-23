void onWsEvent(AsyncWebSocket *server, AsyncWebSocketClient *client, AwsEventType type, void *arg, uint8_t *data, size_t len) {
    String output;
    JsonDocument receivedData;
    if (type == WS_EVT_CONNECT) {
        // Quand un client se connecte, envoyer un message
        Serial.printf("SOCK: Client %u connecté\n", client->id());
        //client->text("Bienvenue sur le serveur WebSocket !");
    } else if (type == WS_EVT_DISCONNECT) {
        // Quand un client se déconnecte
        Serial.printf("SOCK: Client %u déconnecté\n", client->id());
    } else if (type == WS_EVT_DATA) {
        
        // Quand un message est reçu, réagir
        Serial.printf("SOCK: Message reçu de client %u : %.*s\n", client->id(), len, data);
        DeserializationError error = deserializeJson(receivedData, data);
        if (error) {
          Serial.print("Failed to parse JSON: ");
          Serial.println(error.f_str());
          return; // Sortie si une erreur survient
        }
        if (!receivedData.containsKey("ACTION")) {
            Serial.println("ERROR: 'ACTION' key is missing in the received JSON.");
            return;  // Quitte la fonction si "ACTION" est manquant
        }
        const char* action = receivedData["ACTION"];

        if (!receivedData.containsKey("MSG")) {
            Serial.println("ERROR: 'MSG' key is missing in the received JSON.");
            return;  // Quitte la fonction si "MSG" est manquant
        }
        JsonObject message = receivedData["MSG"];

        parseDataFromSocket(action, message);
    }
}

void parseDataFromSocket(const char* action, JsonObject message) {
        // Fusionne le JSON reçu avec 'teams'
        if (strcmp(action,  "PING") == 0) {
        }
        else if (strcmp(action,  "HELLO") == 0) {
          notifyAll();
        }
        else if (strcmp(action,  "FULL") == 0) {
          JsonDocument& doc = teamsAndBumpers;
          doc=message;
        }
        else if (strcmp(action,  "UPDATE") == 0) {
          update("UPDATE", message);
        }
        else if (strcmp(action,  "DELETE") == 0) {
          update("DELETE", message);
        }
        else if (strcmp(action,  "RESET") == 0) {
          resetServer();
        }
        else if (strcmp(action,  "REBOOT") == 0) {
          rebootServer();
        }
        else {
          Serial.printf("SOCK: Action not recognized %s\n", action);
        }
}
