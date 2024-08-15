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
        //JsonObject jsonObj = teamsAndBumpers.as<JsonObject>();
        //JsonObject jsonObjData = receivedData.as<JsonObject>();
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
        if (strcmp(action,  "HELLO") == 0) {
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
          Serial.printf("SOCK: Rebooting....");
          ESP.restart();
        }
        else {
          Serial.printf("SOCK: Action not recognized %s\n", action);
        }

}

// Fonction pour fusionner deux documents JSON
void mergeJson(JsonObject& destObj, const JsonObject& srcObj) {
    //JsonObject destObj = dest.as<JsonObject>();
    //JsonObject srcObj = src.as<JsonObject>();
  String msg="Merging :";
  for (JsonPair kvp : srcObj) {
      if (kvp.value().is<JsonObject>()) {
        JsonObject nestedDestObj;
        if (destObj.containsKey(kvp.key()) && destObj[kvp.key()].is<JsonObject>()) {
                nestedDestObj = destObj[kvp.key()].as<JsonObject>();
            } else {
                //nestedDestObj = destObj.createNestedObject(kvp.key());
                nestedDestObj = destObj[kvp.key()].to<JsonObject>();
            }

        //JsonObject dst = destObj[kvp.key()].as<JsonObject>();
        JsonObject nestedSrcObj  = kvp.value().as<JsonObject>();
        mergeJson(nestedDestObj, nestedSrcObj );
      }
      else {
        destObj[kvp.key()] = kvp.value();
      }
    }

}

void update(String action, JsonObject& obj) {
    JsonObject jsonObj = teamsAndBumpers.as<JsonObject>();
    String output;
      serializeJson(obj, output);
        // Fusionne le JSON reçu avec 'teams'
    //Serial.printf("Update: %s", obj);

    Serial.printf("Update: received %s\n", output.c_str());

    mergeJson(jsonObj, obj);
    serializeJson(jsonObj, output);
        // Fusionne le JSON reçu avec 'teams'
    Serial.printf("Update: complete %s\n", output.c_str());
    
    notifyAll();
}

void notifyAll() {
  String output;
  serializeJson(teamsAndBumpers, output);
  saveJson();
  Serial.printf("SOCK: send to all %s\n", output.c_str());
  ws.textAll(output.c_str());
  sendMessageToAllClients("UPDATE", output.c_str() );
}

void loadJson(String path) {
  String output;
  // Ouvrir le fichier en lecture
  File file = LittleFS.open(path, "r");
  if (!file) {
    Serial.println("Failed to open file for reading. Initializing with default values.");
    // Initialiser avec les valeurs par défaut en cas d'échec d'ouverture du fichier
    File file = LittleFS.open(path+".save", "r");
    if (!file) {
      Serial.println("Failed to open file for reading. Initializing with default values.");
      // Initialiser avec les valeurs par défaut en cas d'échec d'ouverture du fichier
      teamsAndBumpers["bumpers"] = JsonObject();
      teamsAndBumpers["teams"] = JsonObject();
      return;
    }
  }

  // Désérialiser le contenu du fichier dans le JsonDocument
  DeserializationError error = deserializeJson(teamsAndBumpers, file);
  if (error) {
    Serial.print("deserializeJson() failed: ");
    Serial.println(error.f_str());
    // Initialiser avec les valeurs par défaut en cas d'erreur de désérialisation
    teamsAndBumpers["bumpers"] = JsonObject();
    teamsAndBumpers["teams"] = JsonObject();
  } else {
    Serial.println("JSON loaded successfully");
  }

  // Fermer le fichier après utilisation
  file.close();
  serializeJson(teamsAndBumpers, output);
  Serial.printf("JSON: loaded %s\n", output.c_str());
}



void saveJson() {
  // Ouvrir le fichier en écriture
  File file = LittleFS.open("/game.json.save", "w");
  if (!file) {
    Serial.println("Failed to open file for writing");
    return;
  }

  // Sérialiser le JsonDocument dans le fichier
  if (serializeJson(teamsAndBumpers, file) == 0) {
    Serial.println("Failed to write to file");
  }

  // Fermez le fichier après utilisation
  file.close();

  Serial.println("JSON saved successfully");
}

