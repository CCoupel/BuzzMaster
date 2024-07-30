
/*
{ "ID": "<MAC>", "ACTION": "HELLO", "MSG": { "IP": "<ip>"}}

{ "ID": "<MAC>", ACTION": "BUTTON", "MSG": { "button": "<id>", "time": "<epoch>"}}

*/
AsyncServer bumperServer(1234); // Port 1234

void b_handleData(void* arg, AsyncClient* c, void *data, size_t len) {
  JsonDocument receivedData;
  JsonDocument obj;
  String s_data=String((char*)data).substring(0, len);
  String action;
  String bumperID;
  JsonObject MSG;

  Serial.print("BUZZER: Data received: ");
  Serial.println(s_data);
  deserializeJson(receivedData, data);
        //JsonObject jsonObj = teamsAndBumpers.as<JsonObject>();
        JsonObject jsonObjData = receivedData.as<JsonObject>();

  bumperID=jsonObjData["ID"].as<String>();
  action=jsonObjData["ACTION"].as<String>();
  MSG=jsonObjData["MSG"];

  Serial.println("bumperID="+bumperID+" ACTION="+action);

  JsonObject subObj=obj["root"].to<JsonObject>();

  if (action == "HELLO") {
      subObj["bumpers"][bumperID]=MSG;
      update("new", subObj);
      notifyAll();
    }
   if (action ==  "BUTTON") {
      subObj["bumpers"][bumperID]=MSG;
      update("new", subObj);
      notifyAll();
  } 
}


void b_onCLient(void* arg, AsyncClient* client) {
  Serial.println("Buzzer: New client connected");
  client->onData(&b_handleData, NULL);
  /*
  serializeJson(teamsAndBumpers, output);
        ws.textAll(output.c_str());
  */
  
}

void startBumperServer()
{
  bumperServer.onClient(&b_onCLient, NULL);
  
  bumperServer.begin();
    Serial.print("BUMPER server started on port ");
  Serial.println(1234);

}

