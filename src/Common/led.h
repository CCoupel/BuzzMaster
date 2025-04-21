#pragma once
#include <Adafruit_WS2801.h>
#include <SPI.h>

int ledPin = PIN_NEOPIXEL; // Vérifiez la documentation pour la broche LED intégrée sur votre carte ESP32-S3
int rgbPin = RGB_BUILTIN;

#define DATAPIN 7
#define CLOCKPIN 6
#define NUMPIXELS 13

int currentRed = 0;
int currentGreen = 0;
int currentBlue = 0;
int currentIntensity = 255;

Adafruit_WS2801 strip = Adafruit_WS2801(NUMPIXELS, DATAPIN, CLOCKPIN);

void setPixelColor(int led, int r, int g, int b)
{
  strip.setPixelColor(led, r,g, b);
  strip.show();

}

void applyLedColor() {
  int adjustedRed = (currentRed * currentIntensity) / 255;
  int adjustedGreen = (currentGreen * currentIntensity) / 255;
  int adjustedBlue = (currentBlue * currentIntensity) / 255;

  neopixelWrite(rgbPin,adjustedRed,adjustedGreen,adjustedBlue);

  for (int i = 0; i < NUMPIXELS; i++) {
    setPixelColor(i, adjustedRed,adjustedGreen, adjustedBlue);
  }
  
  strip.show();

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

  strip.begin();
  strip.show();

}