

void putMsgToQueue(const char* action, const char* msg, bool notify, AsyncClient* client )
{
  messageQueue_t message;

    message.action = action;
    message.message = msg;
    if (msg == "") { message.message="''"; }
    
    message.notifyAll = notify;
    message.client = client;

    xQueueSend(messageQueue, &message, portMAX_DELAY); // Utilisation en dehors de l'ISR
}

void applyLedColor() {
  int adjustedRed = (currentRed * currentIntensity) / 255;
  int adjustedGreen = (currentGreen * currentIntensity) / 255;
  int adjustedBlue = (currentBlue * currentIntensity) / 255;

#if defined(ESP32)
  neopixelWrite(rgbPin,adjustedRed,adjustedGreen,adjustedBlue);
  int pwmValue = 255 - currentIntensity;
  //analogWrite(LED_BUILTIN, pwmValue);
  //delay(500);
#elif defined(ESP8266)
  int pwmValue = 255 - currentIntensity;
  analogWrite(LED_BUILTIN, pwmValue);
#endif
}

void wifiConnect()
{
  Serial.println();
  Serial.print("Connexion à ");
  Serial.println(ssid);
  
  WiFi.begin(ssid, password);

  while (WiFi.status() != WL_CONNECTED) 
  {
    delay(500);
    Serial.print(".");
  }

  Serial.println("");
  Serial.println("WiFi connecté.");
  Serial.print("Adresse IP: ");
  Serial.println(WiFi.localIP());
  Serial.print("Adresse MAC: ");
  Serial.println(WiFi.macAddress());
}

void listLittleFSFiles() {
    Serial.println("Listing files in LittleFS:");
  #if defined(ESP8266)
    Dir dir = LittleFS.openDir("/");
    while (dir.next()) {
        String fileName = dir.fileName();
        size_t fileSize = dir.fileSize();
        Serial.printf("FILE: %s, SIZE: %d bytes\n", fileName.c_str(), fileSize);
    }
  #elif defined(ESP32)
    File root = LittleFS.open("/");
    if (!root) {
        Serial.println("- failed to open directory");
        return;
    }
    if (!root.isDirectory()) {
        Serial.println(" - not a directory");
        return;
    }

    File file = root.openNextFile();
    while (file) {
        if (file.isDirectory()) {
            Serial.print("DIR : ");
            Serial.println(file.name());
        } else {
            Serial.print("FILE: ");
            Serial.print(file.name());
            Serial.print("\tSIZE: ");
            Serial.println(file.size());
        }
        file = root.openNextFile();
    }
#endif
}

void resetBumpersTime() {
  std::vector<String> keysToRemove = {"STATUS", "BUTTON", "TIME", "DELAY", "DELAY_TEAM"};

  JsonObject bumpers=teamsAndBumpers["bumpers"];
  JsonObject teams=teamsAndBumpers["teams"];
  for (JsonPair kvp : bumpers) {
    if (kvp.value().is<JsonObject>()) {
      JsonObject bumper = kvp.value().as<JsonObject>();

      for (String key : keysToRemove) {
        if (bumper.containsKey(key)) {
          bumper.remove(key);
        }    
      }
    } else {
        Serial.println("Error: Bumper entry is not a JsonObject");
    }
  }
  for (JsonPair team : teams) {
    if (team.value().is<JsonObject>()) {
      JsonObject teamData = team.value().as<JsonObject>();

      for (String key : keysToRemove) {
        if (teamData.containsKey(key)) {
          teamData.remove(key);
        }    
      }
    } else {
        Serial.println("Error: Team entry is not a JsonObject");
    }
  }
  timeRef=0;
  for (const auto& pair : timeRefTeam) {
    timeRefTeam[pair.first]=0;
  }
}

void setLedColor(int red, int green, int blue, bool isApplyLedColor) {

  currentRed = red;
  currentGreen = green;
  currentBlue = blue;
  if (isApplyLedColor) {
    applyLedColor();
  }
}

void setLedIntensity(int intensity) {
  currentIntensity = intensity;
  applyLedColor();
}

void startGame(){
  resetBumpersTime();
  GameStarted=true;
  putMsgToQueue("START","",true);
  
}

void stopGame(){
  GameStarted=false;
  putMsgToQueue("STOP","",true);

}

void pauseGame(AsyncClient* client) {
  putMsgToQueue("PAUSE","",true, client);
}

void attachButtons()
{
  for (size_t id = 0; id < sizeof(buttonsInfo) / sizeof(ButtonInfo); id++) 
  {
    pinMode(buttonsInfo[id].pin, INPUT_PULLUP); 
    attachInterruptArg(digitalPinToInterrupt(buttonsInfo[id].pin),reinterpret_cast<void (*)(void*)>(buttonHandler), &buttonsInfo[id],FALLING);
    Serial.println("Button " + String(id) + " (" + buttonsInfo[id].name + ") attached to pin " + String(buttonsInfo[id].pin));
  }
}


void startBumperServer()
{
  bumperServer = new AsyncServer(1234);
  bumperServer->onClient(&b_onCLientConnect, bumperServer);
  
  bumperServer->begin();
    Serial.print("BUMPER server started on port ");
  Serial.println(1234);

}

void sendMessageToClient(const String& action, const String& msg, AsyncClient* client) {
Serial.println("BUMPER: send to "+client->remoteIP().toString());
Serial.println("BUMPER: action "+action);
Serial.println("BUMPER: message "+msg);
String message="{ \"ACTION\": \"" + action + "\", \"MSG\": " + msg + "}\n";
    if (client && client->connected()) {
      client->write(message.c_str(), message.length()); // Envoie le message au client connecté
    }
}

void sendMessageToAllClients(const String& action, const String& msg ) {
  // Parcourez tous les clients connectés
  Serial.println("BUMPER: send to all");
  for (AsyncClient* client : bumperClients) {
    sendMessageToClient(action, msg, client);
  }
  Serial.println("BUMPER: all is sent");
}

void checkPingForAllClients() {
  unsigned long currentTime = millis();

  // Vérifier les bumpers non répondus
  JsonObject bumpers = teamsAndBumpers["bumpers"];
  for (JsonPair bumperPair : bumpers) {
    JsonObject bumper = bumperPair.value().as<JsonObject>();
    if (currentTime - bumper["lastPingTime"].as<unsigned long>() > 3000) {  // 2 secondes sans réponse
      if (bumper["STATUS"] != "offline") {
        Serial.print("BUMPER: Bumper ");
        Serial.print(bumper["IP"].as<String>());
        Serial.println(" is going offline");
        bumper["STATUS"] = "offline";
        notifyAll();
      }
    }
  }
}

void sendMessageTask(void *parameter) {
    messageQueue_t receivedMessage;

    while (1) {
        // Attendre qu'un message soit disponible dans la file d'attente
        if (xQueueReceive(messageQueue, &receivedMessage, portMAX_DELAY)) {
          Serial.print("BUZZCONTROL: new message in queue: "); 
          if (receivedMessage.action != nullptr) {
            Serial.println(receivedMessage.action);
            if (receivedMessage.action == "START") {
              setLedColor(255, 0, 0);
              setLedIntensity(255);
            }
            if (receivedMessage.action == "STOP") {
              setLedColor(0, 255, 0);
              setLedIntensity(255);
            }
            if (receivedMessage.action == "PAUSE") {
              setLedColor(255, 255, 0);
              setLedIntensity(64);
            }
            // Envoyer le message à travers la socket TCP
            // Vous pouvez remplacer cette partie par votre code de socket
            if (receivedMessage.client != nullptr) {
              sendMessageToClient(receivedMessage.action, receivedMessage.message, receivedMessage.client);
            }
            else {
              sendMessageToAllClients(receivedMessage.action, receivedMessage.message);
            }
          }
          if (receivedMessage.notifyAll == true) {
            notifyAll();
          }
        }
    }
}

void parseDataFromSocket(const char* action, JsonObject message) {
        // Fusionne le JSON reçu avec 'teams'
        if (strcmp(action,  "PING") == 0) {
        }
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
          if (LittleFS.exists(saveGameFile)) {
        // Supprimer le fichier
            if (LittleFS.remove(saveGameFile)) {
                Serial.println("Fichier supprimé avec succès");
            } else {
                Serial.println("Erreur : Impossible de supprimer le fichier");
            }
          } 
          ESP.restart();
        }
        else {
          Serial.printf("SOCK: Action not recognized %s\n", action);
        }

}

// Fonction pour fusionner deux documents JSON
void mergeJson(JsonObject& destObj, const JsonObject& srcObj) {
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
  File file;
  String output;
  // Ouvrir le fichier en lecture
  Serial.print("Loading game file: ");
  if (LittleFS.exists(saveGameFile)) {
    Serial.println(saveGameFile);
    file = LittleFS.open(saveGameFile, "r");
  } else if (LittleFS.exists(GameFile)) {
    Serial.println(GameFile);
    file = LittleFS.open(GameFile, "r");
  }
  if (!file) {
    Serial.println("Failed to open file for reading. Initializing with default values.");
    // Initialiser avec les valeurs par défaut en cas d'échec d'ouverture du fichier
    teamsAndBumpers["bumpers"] = JsonObject();
    teamsAndBumpers["teams"] = JsonObject();
    return;
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
  File file = LittleFS.open(saveGameFile, "w");
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

/* *********** INTERUPTIUONS ************** */
static void IRAM_ATTR buttonHandler(void *arg)
{
    ButtonInfo* buttonInfo = static_cast<ButtonInfo*>(arg);
    switch(buttonInfo->pin) {
      case 0:
        if (GameStarted) {
          stopGame();
        }
        else {
          startGame();
        }
        break;
    };
}

void onTimerISR() {
  sendMessageToAllClients("PING", "'Are you alive?'");  // Appelée toutes les secondes
  checkPingForAllClients();
}

void b_handleData(void* arg, AsyncClient* c, void *data, size_t len) {
  JsonDocument receivedData;
//  JsonDocument obj;
  String s_data=String((char*)data).substring(0, len);
  String action;
  String bumperID;
  JsonObject MSG;
  JsonObject bumpers=teamsAndBumpers["bumpers"];
  JsonObject teams=teamsAndBumpers["teams"];

  Serial.print("BUZZER: Data received: ");
  Serial.println(s_data);
  deserializeJson(receivedData, data);
        JsonObject jsonObjData = receivedData.as<JsonObject>();

  bumperID=jsonObjData["ID"].as<String>();
  action=jsonObjData["ACTION"].as<String>();
  MSG=jsonObjData["MSG"];

  Serial.println("bumperID="+bumperID+" ACTION="+action);
  
  if (action == "HELLO") {
    if ( !bumpers[bumperID].containsKey("NAME") ) {
      bumpers[bumperID]["NAME"]=MSG["NAME"];
    };
    if ( !bumpers[bumperID].containsKey("TEAM") ) {
      bumpers[bumperID]["TEAM"]="";
    };
    bumpers[bumperID]["IP"]=MSG["IP"];
    notifyAll();
  }
  if (action == "BUTTON") {
    if ( bumpers.containsKey(bumperID) ) {

      if ( bumpers[bumperID].containsKey("IP") && bumpers[bumperID]["IP"] == c->remoteIP().toString()) {
        const char * b_team=bumpers[bumperID]["TEAM"];
        int b_time=MSG["time"];
        int b_button=MSG["button"];
        if ( b_team != nullptr ) {
          
          if (timeRef==0) {
            timeRef=b_time;
          }
          int b_delay=b_time-timeRef;

         if (timeRefTeam[b_team]==0) {
            timeRefTeam[b_team]=b_time;
          }

          int b_delayTeam=b_time-timeRefTeam[b_team];
          bumpers[bumperID]["TIME"]=b_time;
          bumpers[bumperID]["BUTTON"]=b_button;
          bumpers[bumperID]["DELAY"]=b_delay;
          bumpers[bumperID]["DELAY_TEAM"]=b_delayTeam;

          teams[b_team]["TIME"]=b_time;
          teams[b_team]["DELAY"]=b_delay;
          teams[b_team]["STATUS"]="PAUSE";
          teams[b_team]["BUMPER"]=bumperID;

          pauseGame(c);
        };
      };
    };
  };
  if (action == "PING") {
    bumpers[bumperID]["lastPingTime"] = millis();  
    if (bumpers[bumperID]["STATUS"] != "online") {
      Serial.println("BUMPER: Bumper is going online");
      bumpers[bumperID]["STATUS"] = "online";
      notifyAll();
    }
  }

};

static void b_onClientDisconnect(void* arg, AsyncClient* client) {
  JsonObject bumpers = teamsAndBumpers["bumpers"];

  Serial.printf("Client disconnected\n");
  for (JsonPair bumperPair : bumpers) {
    JsonObject bumper = bumperPair.value().as<JsonObject>();
    if (bumper["IP"].as<String>() == client->remoteIP().toString()) {
      bumper["STATUS"] = "offline";
      notifyAll();
      break;
    }
  }

  bumperClients.erase(std::remove(bumperClients.begin(), bumperClients.end(), client), bumperClients.end()); // Retirer le client de la liste
  delete client; // Nettoyer la mémoire
  
}


static void b_onCLientConnect(void* arg, AsyncClient* client) {
  Serial.println("Buzzer: New client connected");
  client->onData(&b_handleData, NULL);
  client->onDisconnect(&b_onClientDisconnect, NULL);
  bumperClients.push_back(client);

}

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