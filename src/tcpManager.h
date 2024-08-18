void startBumperServer()
{
  bumperServer = new AsyncServer(1234);
  bumperServer->onClient(&b_onCLientConnect, bumperServer);
  
  bumperServer->begin();
    Serial.print("BUMPER server started on port ");
  Serial.println(1234);

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
