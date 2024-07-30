




void w_handleRoot(AsyncWebServerRequest *request) {
  Serial.println("WWW: ROOT");
  digitalWrite(led, 1);
  request->send(200, "text/plain", "hello from esp8266!");
  digitalWrite(led, 0);
}


void w_handle_bumpers(AsyncWebServerRequest *request) {
  String output;
    serializeJson(bumpers, output);
  
  request->send(200, "text/json", output);
}

void w_handle_teams(AsyncWebServerRequest *request) {
  String output;
    serializeJson(teams, output);
  
  request->send(200, "text/json", output);
}

void w_inline(AsyncWebServerRequest *request) {
  Serial.println("WWW: inline");
  request->send(200, "text/plain", "this works as well");
}


void w_handleNotFound(AsyncWebServerRequest *request) {
  Serial.println("WWW: not found");
  digitalWrite(led, 1);
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
  digitalWrite(led, 0);
}

void startWebServer() {
  server.serveStatic("/fs", LittleFS, "/").setDefaultFile("index.html");

  server.on("/bumpers", HTTP_GET, w_handle_bumpers);

  server.on("/teams", HTTP_GET, w_handle_teams);

  server.on("/", HTTP_GET, w_handleRoot);
  server.onNotFound(w_handleNotFound);
  
  ws.onEvent(onWsEvent);
  server.addHandler(&ws);

  server.begin();
  Serial.println("HTTP server started");
}

