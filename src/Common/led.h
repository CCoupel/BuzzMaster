#pragma once

int ledPin = PIN_NEOPIXEL; // Vérifiez la documentation pour la broche LED intégrée sur votre carte ESP32-S3
int rgbPin = RGB_BUILTIN;

int currentRed = 0;
int currentGreen = 0;
int currentBlue = 0;
int currentIntensity = 255;

void applyLedColor() {
  int adjustedRed = (currentRed * currentIntensity) / 255;
  int adjustedGreen = (currentGreen * currentIntensity) / 255;
  int adjustedBlue = (currentBlue * currentIntensity) / 255;

  neopixelWrite(rgbPin,adjustedRed,adjustedGreen,adjustedBlue);

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
}