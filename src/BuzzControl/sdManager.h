#pragma once

#include <Arduino.h>
#include <SD.h>
#include <SPI.h>
#include "Common/CustomLogger.h"


// Définition des broches SPI pour la carte SD
#define SD_CS    10  // Broche CS (Chip Select)
#define SD_SCK   12  // Broche SCK

#define SD_MOSI  11  // Broche MOSI
#define SD_MISO  13  // Broche MISO

static const char* SD_TAG = "SD_MANAGER";

String sdInfo() {
    String result="";
    uint8_t cardType = SD.cardType();

    result += "Type de carte SD : \n";
    if(cardType == CARD_MMC){
        result += "MMC\n";
    } else if(cardType == CARD_SD){
        result +="SDSC\n";
    } else if(cardType == CARD_SDHC){
        result +="SDHC\n";
    } else {
        result +="INCONNU\n";
    }

    uint64_t cardSize = SD.cardSize() / (1024 * 1024);
    result +="Taille de la carte: ";
    result += String(cardSize) + "MB\n";

    return result;
}


void sdInit() {
  SPI.begin();
  ESP_LOGI(SD_TAG,"SPI initialised");
  SPI.setFrequency(1000000); // Réduire la fréquence à 400kHz
  SPI.setDataMode(SPI_MODE0);

  if (!SD.begin(SD_CS, SPI)) {
    ESP_LOGW(SD_TAG,"Can't initialize SD CARD");
    return;
  }
  ESP_LOGI(SD_TAG,"SD initialised");
    // Affichage des informations de la carte
  uint8_t cardType = SD.cardType();
  if(cardType == CARD_NONE){
    ESP_LOGW(SD_TAG,"no SD détected");
    return;
  }

  ESP_LOGI(SD_TAG,sdInfo().c_str());
}

