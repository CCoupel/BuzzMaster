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

volatile int currentRed = 0;
volatile int currentGreen = 0;
volatile int currentBlue = 0;
volatile int currentIntensity = 255;
int currentOffset = 0;
int currentPeriod = 1;

//Adafruit_WS2801 strip_ws28 = Adafruit_WS2801(NUMPIXELS, DATAPIN_ws28, CLOCKPIN_ws28);
Adafruit_DotStar strip_sk98 = Adafruit_DotStar(NUMPIXELS, DATAPIN_sk98, CLOCKPIN_sk98, DOTSTAR_BGR);

void showPixels() {
 // strip_ws28.show();
  strip_sk98.show();

}

void setPixelColor(int led, int r, int g, int b)
{
  //strip_ws28.setPixelColor(led/2, r,g, b);
  strip_sk98.setPixelColor(led, r,g, b);
  showPixels();
}

// Populate the LED buffer with the current background colour without calling
// showPixels(). Use before overlay operations so the intermediate full-background
// state is never pushed to the hardware (avoids a visible flash on each tick).
void applyLedColorToBuffer() {
  int adjustedRed = (currentRed * currentIntensity) / 255;
  int adjustedGreen = (currentGreen * currentIntensity) / 255;
  int adjustedBlue = (currentBlue * currentIntensity) / 255;

  neopixelWrite(rgbPin, adjustedRed, adjustedGreen, adjustedBlue);

  for (int i = 0; i < NUMPIXELS; i++) {
    if (currentPeriod <= 1 || (i % currentPeriod == currentOffset % currentPeriod)) {
      strip_sk98.setPixelColor(i, adjustedRed, adjustedGreen, adjustedBlue);
    } else {
      strip_sk98.setPixelColor(i, 0, 0, 0);
    }
  }
}

void applyLedColor() {
  applyLedColorToBuffer();
  showPixels();
}

void setLedColor(int red, int green, int blue, bool isApplyLedColor = false, int offset = 0, int period = 1) {
  currentRed = red;
  currentGreen = green;
  currentBlue = blue;
  currentOffset = offset;
  currentPeriod = period;
  if (isApplyLedColor) {
    applyLedColor();
  }
}

// Set LED color excluding every Nth LED (for 3/4 pattern: period=4 lights all except every 4th)
void setLedColorExclude(int red, int green, int blue, int period) {
  int adjustedRed = (red * currentIntensity) / 255;
  int adjustedGreen = (green * currentIntensity) / 255;
  int adjustedBlue = (blue * currentIntensity) / 255;

  neopixelWrite(rgbPin, adjustedRed, adjustedGreen, adjustedBlue);

  for (int i = 0; i < NUMPIXELS; i++) {
    if (i % period != (period - 1)) {
      strip_sk98.setPixelColor(i, adjustedRed, adjustedGreen, adjustedBlue);
    } else {
      strip_sk98.setPixelColor(i, 0, 0, 0);
    }
  }
  showPixels();

  currentRed = red;
  currentGreen = green;
  currentBlue = blue;
}

void setLedIntensity(int intensity) {
  currentIntensity = intensity;
  applyLedColor();
}

void initLED() {
  setLedColor(128, 128, 128);
  setLedIntensity(128);

  //strip_ws28.begin();
  strip_sk98.begin();
  showPixels();

}