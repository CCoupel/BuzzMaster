
AsyncServer bumperServer(1234); // Port 1234

void b_handleData(void* arg, AsyncClient* c, void *data, size_t len) {
  String s_data=String((char*)data).substring(0, len);

  Serial.print("Data received from client: ");
  Serial.println(s_data);
}


void b_onCLient(void* arg, AsyncClient* client) {
  Serial.println("New client connected");
  client->onData(&b_handleData, NULL);
}

void startBumperServer()
{
  bumperServer.onClient(&b_onCLient, NULL);
  
  bumperServer.begin();
    Serial.print("BUMPER server started on port ");
  Serial.println(1234);

}
