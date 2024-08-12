
/*
{ "ID": "<MAC>", "ACTION": "HELLO", "MSG": { "IP": "<ip>"}}

{ "ID": "<MAC>", ACTION": "BUTTON", "MSG": { "button": "<id>", "time": "<epoch>"}}

*/
AsyncServer* bumperServer;
std::vector<AsyncClient*> bumperClients;



void startGame(){
  resetBumpersTime();
  GameStarted=true;
  digitalWrite(ledPin, LOW);
  sendMessageToAllClients("START","''");
  notifyAll();
}

void stopGame(){
  GameStarted=false;
  digitalWrite(ledPin, HIGH);
  sendMessageToAllClients("STOP","''");
  notifyAll();
}

void pauseGame(AsyncClient* client) {
    sendMessageToClient("PAUSE", "''", client);
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
        bumpers[bumperID]["TEAM"]="1";
      };
      bumpers[bumperID]["IP"]=MSG["IP"];
      
//      update("new", subObj);
      notifyAll();
  };
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
      //update("new", subObj);
          //notifyAll();
          pauseGame(c);
        };
      };
    };
  }; 
};

static void b_onClientDisconnect(void* arg, AsyncClient* client) {
  Serial.printf("Client disconnected\n");
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
String message="{ ACTION: '" + action + "', MSG: " + msg + "}";

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
}

