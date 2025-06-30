// Define the LED pins
const int redPin = 7;
const int yellowPin = 10;
const int greenPin = 12;

char currentState = 'A';  // Start with Absent state

void setup() {
  Serial.begin(9600);

  // Initialize pins
  pinMode(redPin, OUTPUT);
  pinMode(yellowPin, OUTPUT);
  pinMode(greenPin, OUTPUT);

  setAbsent();

}

void loop() {

// Check if data is available
  if (Serial.available() > 0) {
    // Read incoming byte
    char incomingState = Serial.read();
    
    // Only process valid states
    if (incomingState == 'F' || incomingState == 'D' || incomingState == 'A') {
      if (incomingState != currentState) {
        currentState = incomingState;
        
        // Set LED state based on received state
        switch (currentState) {
          case 'F':  // Focused
            setFocused();
            break;
          case 'D':  // Distracted
            setDistracted();
            break;
          case 'A':  // Absent
            setAbsent();
            break;
        }
        
        // Send confirmation back
        Serial.print("State changed to: ");
        Serial.println(currentState);
      }
    }
  }
}

void setFocused() {
  digitalWrite(redPin, LOW);
  digitalWrite(yellowPin, LOW);
  digitalWrite(greenPin, HIGH);
}

void setAbsent() {
  digitalWrite(redPin, LOW);
  digitalWrite(yellowPin, HIGH);
  digitalWrite(greenPin, LOW);
}

void setDistracted() {
  digitalWrite(redPin, HIGH);
  digitalWrite(yellowPin, LOW);
  digitalWrite(greenPin, LOW);
}