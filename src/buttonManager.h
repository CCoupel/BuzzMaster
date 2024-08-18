static void IRAM_ATTR buttonHandler(void *arg)
{
    ButtonInfo* buttonInfo = static_cast<ButtonInfo*>(arg);
    switch(buttonInfo->pin) {
      case 0:
        if (GameStarted) {
          stopGame();
        }
        else {
          startGame();
        }
        break;
    };
}

void attachButtons()
{
  for (size_t id = 0; id < sizeof(buttonsInfo) / sizeof(ButtonInfo); id++) 
  {
    pinMode(buttonsInfo[id].pin, INPUT_PULLUP); 
    attachInterruptArg(digitalPinToInterrupt(buttonsInfo[id].pin),reinterpret_cast<void (*)(void*)>(buttonHandler), &buttonsInfo[id],FALLING);
    Serial.println("Button " + String(id) + " (" + buttonsInfo[id].name + ") attached to pin " + String(buttonsInfo[id].pin));
  }
}