#include <esp_log.h>
static const char* BUTTON_TAG = "BUTTON_MANAGER";

static void IRAM_ATTR buttonHandler(void *arg) {
    ButtonInfo* buttonInfo = static_cast<ButtonInfo*>(arg);
    switch(buttonInfo->pin) {
        case 0:
            if (GameStarted) {
                stopGame();
            } else {
                startGame();
            }
            break;
        default:
            ESP_LOGW(BUTTON_TAG, "Unknown button pin: %d", buttonInfo->pin);
            break;
    }
}

void attachButtons()
{
  for (size_t id = 0; id < sizeof(buttonsInfo) / sizeof(ButtonInfo); id++) 
  {
    pinMode(buttonsInfo[id].pin, INPUT_PULLUP); 
    attachInterruptArg(digitalPinToInterrupt(buttonsInfo[id].pin),reinterpret_cast<void (*)(void*)>(buttonHandler), &buttonsInfo[id],FALLING);
    ESP_LOGI(BUTTON_TAG, "Button %d (%s) attached to pin %d", id, buttonsInfo[id].name.c_str(), buttonsInfo[id].pin);
  }
}