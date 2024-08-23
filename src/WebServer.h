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

void w_handleRoot(AsyncWebServerRequest *request) {
  Serial.println("WWW: ROOT");
  digitalWrite(ledPin, HIGH);
  request->send(200, "text/plain", "hello from esp8266!");
  digitalWrite(ledPin, LOW);
}

/*
void w_handle_bumpers(AsyncWebServerRequest *request) {
  JsonObject bumpers=teamsAndBumpers["bumpers"];
  JsonObject teams=teamsAndBumpers["teams"];
  String output;
    serializeJson(bumpers, output);
  
  request->send(200, "text/json", output);
}

void w_handle_teams(AsyncWebServerRequest *request) {
  JsonObject bumpers=teamsAndBumpers["bumpers"];
  JsonObject teams=teamsAndBumpers["teams"];
  String output;
    serializeJson(teams, output);
  
  request->send(200, "text/json", output);
}

void w_inline(AsyncWebServerRequest *request) {
  Serial.println("WWW: inline");
  request->send(200, "text/plain", "this works as well");
}
*/

void w_handleNotFound(AsyncWebServerRequest *request) {
  Serial.println("WWW: not found");
  digitalWrite(ledPin, HIGH);
  String message = "File Not Found\n\n";
  message += "URI: ";
  message += request->url();
  message += "\nMethod: ";
  message += (request->method() == HTTP_GET)?"GET":"POST";
  message += "\nArguments: ";
  message += request->args();
  message += "\n";
  for (uint8_t i=0; i<request->args(); i++) {
    message += " " + request->argName(i) + ": " + request->arg(i) + "\n";
  }
  request->send(404, "text/plain", message);
  digitalWrite(ledPin, LOW);
}

void w_handleReboot(AsyncWebServerRequest *request) {
  rebootServer();
}

void w_handleReset(AsyncWebServerRequest *request) {
  resetServer();
}

void startWebServer() {
  server.serveStatic("/", LittleFS, "/").setDefaultFile("index.html");

  server.on("/reset", HTTP_GET, w_handleReset);

  server.on("/reboot", HTTP_GET, w_handleReboot);

  //server.on("/", HTTP_GET, w_handleRoot);
  server.onNotFound(w_handleNotFound);
  
  ws.onEvent(onWsEvent);
  server.addHandler(&ws);

  server.begin();
  Serial.println("HTTP server started");
}

