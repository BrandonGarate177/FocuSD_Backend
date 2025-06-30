import sys
import json
import cv2
import os

def classify_attention(image_path):
    # Load the pre-trained Haar cascade for face detection
    cascade_path = os.path.join(cv2.data.haarcascades, 'haarcascade_frontalface_default.xml')
    face_cascade = cv2.CascadeClassifier(cascade_path)

    # Load the image
    image = cv2.imread(image_path)
    if image is None:
        return {"error": "Could not read image"}

    # Convert to grayscale for face detection
    gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)

    # Detect faces
    faces = face_cascade.detectMultiScale(gray, 1.1, 4)

    if len(faces) > 0:
        # If a face is detected, assume attentive (for demo)
        return {"status": "attentive", "confidence": 0.95}
    else:
        return {"status": "not_attentive", "confidence": 0.10}

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print(json.dumps({"error": "No image provided"}))
        sys.exit(1)
    image_path = sys.argv[1]
    result = classify_attention(image_path)
    print(json.dumps(result))