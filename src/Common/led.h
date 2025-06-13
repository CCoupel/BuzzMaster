#pragma once
#include <Adafruit_WS2801.h>
#include <Adafruit_DotStar.h>

#include <SPI.h>

int ledPin = PIN_NEOPIXEL; // Vérifiez la documentation pour la broche LED intégrée sur votre carte ESP32-S3
int rgbPin = RGB_BUILTIN;

#define DATAPIN_ws28 5
#define CLOCKPIN_ws28 6

#define DATAPIN_sk98 4
#define CLOCKPIN_sk98 3

#define NUMPIXELS 23

int currentRed = 0;
int currentGreen = 0;
int currentBlue = 0;
int currentIntensity = 255;

//Adafruit_WS2801 strip_ws28 = Adafruit_WS2801(NUMPIXELS, DATAPIN_ws28, CLOCKPIN_ws28);
Adafruit_DotStar strip_sk98 = Adafruit_DotStar(NUMPIXELS, DATAPIN_sk98, CLOCKPIN_sk98, DOTSTAR_BGR);

void showPixels() {
  strip_ws28.show();
  strip_sk98.show();

}

void setPixelColor(int led, int r, int g, int b)
{
  //strip_ws28.setPixelColor(led/2, r,g, b);
  strip_sk98.setPixelColor(led, r,g, b);
  showPixels();
}

void applyLedColor() {
  int adjustedRed = (currentRed * currentIntensity) / 255;
  int adjustedGreen = (currentGreen * currentIntensity) / 255;
  int adjustedBlue = (currentBlue * currentIntensity) / 255;

  neopixelWrite(rgbPin,adjustedRed,adjustedGreen,adjustedBlue);

  for (int i = 0; i < NUMPIXELS; i++) {
    setPixelColor(i, adjustedRed,adjustedGreen, adjustedBlue);
  }
  showPixels();
}

void setLedColor(int red, int green, int blue, bool isApplyLedColor = false) {

  currentRed = red;
  currentGreen = green;
  currentBlue = blue;
  if (isApplyLedColor) {
    applyLedColor();
  }
}

void setLedIntensity(int intensity) {
  currentIntensity = intensity;
  applyLedColor();
}

void initLED() {
  setLedColor(0, 0, 0);
  setLedIntensity(0);

  //strip_ws28.begin();
  strip_sk98.begin();
  showPixels();

}