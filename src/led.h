void setLedColor(int red, int green, int blue, bool isApplyLedColor) {

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

void applyLedColor() {
  int adjustedRed = (currentRed * currentIntensity) / 255;
  int adjustedGreen = (currentGreen * currentIntensity) / 255;
  int adjustedBlue = (currentBlue * currentIntensity) / 255;

#if defined(ESP32)
  neopixelWrite(rgbPin,adjustedRed,adjustedGreen,adjustedBlue);
  int pwmValue = 255 - currentIntensity;
  //analogWrite(LED_BUILTIN, pwmValue);
  //delay(500);
#elif defined(ESP8266)
  int pwmValue = 255 - currentIntensity;
  analogWrite(LED_BUILTIN, pwmValue);
#endif
}
