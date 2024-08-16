#include "includes.h"
#include "BumperServer.h"
#include "WebServer.h"


void setup(void)
{
  Serial.begin(115200);

#if defined(ESP32)

#elif defined(ESP8266)
  // Initialiser les broches comme sorties pour ESP8266
  pinMode(LED_BUILTIN, OUTPUT);  
#endif

  #if defined(ESP32)
    Serial.print("RGB pin:");
    Serial.println(RGB_BUILTIN);
  #endif
  Serial.print("LED pin:");
  Serial.println(LED_BUILTIN);
  #if defined(ESP32)
    Serial.print("NEO pin:");
    Serial.println(PIN_NEOPIXEL);
  #endif

  setLedColor(255, 0, 0);
  setLedIntensity(128);

  delay(10);
  wifiConnect();
  timeClient.update();

  if (MDNS.begin("buzzcontrol")) {
    Serial.println("MDNS responder started");
  }
  MDNS.addService("buzzcontrol", "tcp", localWWWpPort);
  MDNS.addService("http", "tcp", localWWWpPort);
  MDNS.addService("sock", "tcp", localUdpPort);

  if (!LittleFS.begin()) {
    Serial.println("Erreur de montage LittleFS");
    return;
  }

  listLittleFSFiles();

  attachButtons();
  loadJson(GameFile);

  startWebServer();
  startBumperServer();
  setLedColor(0, 255, 0);
  setLedIntensity(64);
  messageQueue = xQueueCreate(10, sizeof(messageQueue_t));

    if (messageQueue == NULL) {
      Serial.println("Échec de la création de la file d'attente");
      return;
    }

  xTaskCreate(sendMessageTask, "Send Message Task", 4096, NULL, 1, NULL);
}

void loop(void)
{
#if defined(ESP8266)
//  server.handleClient(); // Gestion des requêtes clients
  MDNS.update();
#endif
}
