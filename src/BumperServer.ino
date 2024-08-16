
/*
{ "ID": "<MAC>", "ACTION": "HELLO", "MSG": { "IP": "<ip>"}}

{ "ID": "<MAC>", ACTION": "BUTTON", "MSG": { "button": "<id>", "time": "<epoch>"}}

*/
AsyncServer* bumperServer;
std::vector<AsyncClient*> bumperClients;

void putMsgToQueue(const char* action=nullptr, const char* msg="", bool notify=false, AsyncClient* client=nullptr )
{
  messageQueue_t message;

    message.action = action;
    message.message = msg;
    if (msg == "") { message.message="''"; }
    
    message.notifyAll = notify;
    message.client = client;

    xQueueSend(messageQueue, &message, portMAX_DELAY); // Utilisation en dehors de l'ISR
}



void startGame(){
  
  //Serial.println("BUZZCONTROL: Starting Game");
  resetBumpersTime();
  GameStarted=true;
  //digitalWrite(ledPin, LOW);
  //setLedColor(255, 0, 0,true);
  //setLedIntensity(255);
  
  putMsgToQueue("START","",true);
  
  /*
  sendMessageToAllClients("START","''");
  notifyAll();
  */
}

void stopGame(){
  //Serial.println("BUZZCONTROL: Stopping Game");
  GameStarted=false;
  //digitalWrite(ledPin, HIGH);
  //setLedColor(0, 255, 0,true);
  //setLedIntensity(64);
  
  putMsgToQueue("STOP","",true);
  /*/
  sendMessageToAllClients("STOP","''");
  notifyAll();
  */
}

void pauseGame(AsyncClient* client) {
//  Serial.println("BUZZCONTROL: Pausing TEAM Game");
  //setLedColor(255, 255, 0);
  //setLedIntensity(64);

  putMsgToQueue("PAUSE","",true, client);
  //sendMessageToClient("PAUSE", "''", client);
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
        //JsonObject jsonObj = teamsAndBumpers.as<JsonObject>();
        JsonObject jsonObjData = receivedData.as<JsonObject>();

  bumperID=jsonObjData["ID"].as<String>();
  action=jsonObjData["ACTION"].as<String>();
  MSG=jsonObjData["MSG"];

  Serial.println("bumperID="+bumperID+" ACTION="+action);

//  JsonObject subObj=obj["root"].to<JsonObject>();
  
  if (action == "HELLO") {
      //subObj["bumpers"][bumperID]=MSG;
//      if ( !bumpers.containsKey(bumperID) ) {
//       bumpers[bumperID]=["bumpers"].to<JsonObject>();
//      };
      if ( !bumpers[bumperID].containsKey("NAME") ) {
        bumpers[bumperID]["NAME"]=MSG["NAME"];
      };
      if ( !bumpers[bumperID].containsKey("TEAM") ) {
        bumpers[bumperID]["TEAM"]="";
      };
      bumpers[bumperID]["IP"]=MSG["IP"];
      
//      update("new", subObj);
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

      //update("new", subObj);
          //notifyAll();
          pauseGame(c);
        };
      };
    };
  };
  if (action == "PING") {
    bumpers[bumperID]["lastPingTime"] = millis();  
    //bumpers[bumperID]["STATUS"] = "online";  // Marquer le bumper comme "online" lors de la réception d'un PONG
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
