#pragma once

#include <ESPAsyncWebServer.h>

static const char* WEB_TAG = "WEBSERVER";

void w_handleRoot(AsyncWebServerRequest *request) {
  ESP_LOGI(WEB_TAG, "Handling root request");
  request->send(200, "text/plain", "hello from BuzzControl!");
  digitalWrite(ledPin, LOW);
}

void w_handleNotFound(AsyncWebServerRequest *request) {
  ESP_LOGW(WEB_TAG, "Handling 404 for: %s", request->url().c_str());
  String message = "File Not Found\n\n";
  message += "URI: " + request->url() + "\n";
  message += "Method: " + String((request->method() == HTTP_GET) ? "GET" : "POST") + "\n";
  message += "Arguments: " + String(request->args()) + "\n";
  
  for (uint8_t i = 0; i < request->args(); i++) {
      message += " " + request->argName(i) + ": " + request->arg(i) + "\n";
  }
  
  request->send(404, "text/plain", message);
}

void w_handleReboot(AsyncWebServerRequest *request) {
    ESP_LOGI(WEB_TAG, "Handling reboot request");
    rebootServer();
}

void w_handleReset(AsyncWebServerRequest *request) {
    ESP_LOGI(WEB_TAG, "Handling reset request");
    resetServer();
}


void w_handleRedirect(AsyncWebServerRequest *request) {
    ESP_LOGD(WEB_TAG, "REdirecting for: %s", request->url().c_str());
    request->redirect("http://buzzcontrol.local/html/config.html");
}

void startWebServer() {
    String ROOT="/";
    if (LittleFS.exists("/CURRENT")) {
        ESP_LOGD(FS_TAG, "Directory /CURRENT Exists");
        ROOT="/CURRENT";
    }

    server.serveStatic("/", LittleFS, ROOT.c_str());

    server.on("/index.html", w_handleRedirect);

    server.on("/generate_204", w_handleRedirect);  // Android captive portal
    server.on("/gen_204", w_handleRedirect);       // Android captive portal
    server.on("/hotspot-detect.html", w_handleRedirect);  // iOS captive portal
    server.on("/canonical.html", w_handleRedirect);       // Windows captive portal
    server.on("/success.txt", w_handleRedirect);          // macOS captive portal
    server.on("/favicon.ico", w_handleRedirect);   // Browser icon

    server.on("/reset", HTTP_GET, w_handleReset);
    server.on("/reboot", HTTP_GET, w_handleReboot);
    
    ws.onEvent(onWsEvent);
    server.addHandler(&ws);

    server.begin();
    ESP_LOGI(WEB_TAG, "HTTP server started on %s", ROOT);
}