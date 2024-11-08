#pragma once
#include "Common/CustomLogger.h"
#include "Common/led.h"

#include <ESPAsyncWebServer.h>

static const char* WEB_TAG = "WEBSERVER";

void addCorsHeaders(AsyncWebServerResponse* response) {
    response->addHeader("Access-Control-Allow-Origin", "*");
    response->addHeader("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS");
    response->addHeader("Access-Control-Allow-Headers", "Content-Type,Authorization");
    response->addHeader("Access-Control-Allow-Credentials", "true");
}

void w_handleRoot(AsyncWebServerRequest *request) {
  ESP_LOGI(WEB_TAG, "Handling root request");
  AsyncWebServerResponse *response = request->beginResponse(200, "text/html", "hello from BuzzControl!");
    addCorsHeaders(response);

  request->send(response);

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
  
  AsyncWebServerResponse *response = request->beginResponse(404, "text/plain", message);
    addCorsHeaders(response);

  request->send(response);
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

void w_handleUploadComplete(AsyncWebServerRequest *request) {
    AsyncWebServerResponse *response = request->beginResponse(200, "text/plain", "Upload terminé");
    addCorsHeaders(response);

    request->send(response);
}


void w_handleUploadFile(AsyncWebServerRequest *request, String filename, size_t index, uint8_t *data, size_t len, bool final) {
    static File file;
    static size_t totalSize = 0;
    if(!index) { // Début de l'upload
        ESP_LOGI(WEB_TAG,"Début de l'upload du Background");
        file = LittleFS.open("/files/background.jpg", "w");
        totalSize=0;
        if(!file) {
            ESP_LOGI(WEB_TAG,"Échec de l'ouverture du fichier background en écriture");
            return;
        }
    }

    if(file && len) { // Écriture des données
        file.write(data, len);
        totalSize+=len;
    }

    if(final) { // Fin de l'upload
        if(file) {
            ESP_LOGD(WEB_TAG,"Upload background terminé: %i", totalSize);
            file.close();
        }
    }
}



void startWebServer() {
    String ROOT="/";
    if (LittleFS.exists("/CURRENT")) {
        ESP_LOGD(FS_TAG, "Directory /CURRENT Exists");
        ROOT="/CURRENT";
    }

    server.serveStatic("/", LittleFS, ROOT.c_str());
    server.onNotFound(w_handleNotFound);

    server.on("/index.html", w_handleRedirect);

    server.on("/generate_204", w_handleRedirect);  // Android captive portal
    server.on("/gen_204", w_handleRedirect);       // Android captive portal
    server.on("/hotspot-detect.html", w_handleRedirect);  // iOS captive portal
    server.on("/canonical.html", w_handleRedirect);       // Windows captive portal
    server.on("/success.txt", w_handleRedirect);          // macOS captive portal
    server.on("/favicon.ico", w_handleRedirect);   // Browser icon

    server.on("/reset", HTTP_GET, w_handleReset);
    server.on("/reboot", HTTP_GET, w_handleReboot);
    
    server.on("/background", HTTP_POST, w_handleUploadComplete, w_handleUploadFile);

    ws.onEvent(onWsEvent);
    server.addHandler(&ws);

    server.begin();
    ESP_LOGI(WEB_TAG, "HTTP server started on %s", ROOT);
}