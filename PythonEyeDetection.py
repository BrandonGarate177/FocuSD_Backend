import cv2
import mediapipe as mp
import numpy as np
import time
from datetime import datetime
from serial import Serial

# -----------------------------
# Step 1: Initialize MediaPipe
# MediaPipe is a library that provides pre-trained ML models for face and body tracking
# We're using Face Mesh which gives us detailed facial landmarks

# Initialize face mesh detector from MediaPipe
mp_face_mesh = mp.solutions.face_mesh
mp_drawing = mp.solutions.drawing_utils
mp_drawing_styles = mp.solutions.drawing_styles

# Configure face mesh with appropriate settings
face_mesh = mp_face_mesh.FaceMesh(
    max_num_faces=1,  # Only detect one face
    refine_landmarks=True,  # Get more detailed landmarks for eyes
    min_detection_confidence=0.5,  # How confident the detection should be
    min_tracking_confidence=0.5  # How confident the tracking should be
)

# ARDUINO CONNECTION
arduino_connected = False
arduino = None

def connect_to_arduino():
    global arduino, arduino_connected

    port = '/dev/cu.usbmodem101'

    try:
        print(f"Trying to connect to Arduino on {port}...")
        ser = Serial(port, 9600)  # Use Serial instead of serial.Serial
        time.sleep(2)
        arduino = ser
        arduino_connected = True
        print(f"Connected to Arduino on {port}.")
        return True
    except Exception as e:
        print(f"Failed to connect to Arduino: {e}")
        arduino_connected = False
        return False

connect_to_arduino()

def send_to_arduino(state):
    #F is focused   A is absent     D is distracted
    if arduino and arduino_connected:
        try:
            arduino.write(state.encode())
            print(f"Sent '{state}' to Arduino.")
        except Exception as e:
            print(f"Error sending to Arduino: {e}")


# -----------------------------
# Step 2: Define landmark indices
# These are points on the face that MediaPipe can track
# Each number corresponds to a specific point on the face

# Points that form the outline of the eyes
LEFT_EYE = [362, 382, 381, 380, 374, 373, 390, 249, 263, 466, 388, 387, 386, 385, 384, 398]
RIGHT_EYE = [33, 7, 163, 144, 145, 153, 154, 155, 133, 173, 157, 158, 159, 160, 161, 246]

# Points that form the outline of the iris (colored part of eye)
LEFT_IRIS = [474, 475, 476, 477]
RIGHT_IRIS = [469, 470, 471, 472]


# -----------------------------
# Step 3: Define helper functions
# -----------------------------

def calculate_eye_aspect_ratio(landmarks, eye_indices):
    """
    Calculate the eye aspect ratio (EAR) to detect if the eye is open

    EAR = (height1 + height2) / (2 * width)

    A lower EAR means the eye is more closed
    """
    # Extract eye landmark points
    eye_points = np.array([[landmarks[i].x, landmarks[i].y] for i in eye_indices])

    # Calculate the horizontal distance (width of eye)
    horizontal_dist = np.linalg.norm(eye_points[0] - eye_points[8])

    # Calculate the vertical distances (height of eye)
    vertical_dist1 = np.linalg.norm(eye_points[4] - eye_points[12])
    vertical_dist2 = np.linalg.norm(eye_points[5] - eye_points[11])

    # Calculate EAR
    ear = (vertical_dist1 + vertical_dist2) / (2.0 * horizontal_dist)

    return ear


def analyze_gaze_direction(landmarks, image):
    """
    Analyze where the eyes are looking by calculating the position of the iris
    relative to the eye center
    """
    h, w, c = image.shape  # Get image dimensions

    # Extract iris positions
    left_iris = np.mean([[landmarks[i].x, landmarks[i].y] for i in LEFT_IRIS], axis=0)
    right_iris = np.mean([[landmarks[i].x, landmarks[i].y] for i in RIGHT_IRIS], axis=0)

    # Extract eye positions
    left_eye = np.array([[landmarks[i].x, landmarks[i].y] for i in LEFT_EYE])
    right_eye = np.array([[landmarks[i].x, landmarks[i].y] for i in RIGHT_EYE])

    # Calculate eye centers
    left_eye_center = np.mean(left_eye, axis=0)
    right_eye_center = np.mean(right_eye, axis=0)

    # Calculate eye widths for normalization
    left_eye_width = np.linalg.norm(left_eye[0] - left_eye[8])
    right_eye_width = np.linalg.norm(right_eye[0] - right_eye[8])

    # Calculate relative iris positions
    # Negative x value means looking left, positive means looking right
    # Negative y value means looking up, positive means looking down
    left_gaze_x = (left_iris[0] - left_eye_center[0]) / left_eye_width
    right_gaze_x = (right_iris[0] - right_eye_center[0]) / right_eye_width

    left_gaze_y = (left_iris[1] - left_eye_center[1]) / (left_eye_width / 2)
    right_gaze_y = (right_iris[1] - right_eye_center[1]) / (right_eye_width / 2)

    # Average both eyes
    gaze_x = (left_gaze_x + right_gaze_x) / 2
    gaze_y = (left_gaze_y + right_gaze_y) / 2

    # Draw visualization points
    # Convert normalized coordinates to pixel coordinates
    left_eye_center_px = (int(left_eye_center[0] * w), int(left_eye_center[1] * h))
    right_eye_center_px = (int(right_eye_center[0] * w), int(right_eye_center[1] * h))
    left_iris_px = (int(left_iris[0] * w), int(left_iris[1] * h))
    right_iris_px = (int(right_iris[0] * w), int(right_iris[1] * h))

    # Draw eye centers (white dots) and iris centers (green dots)
    cv2.circle(image, left_eye_center_px, 3, (255, 255, 255), -1)  # White dot
    cv2.circle(image, right_eye_center_px, 3, (255, 255, 255), -1)  # White dot
    cv2.circle(image, left_iris_px, 3, (0, 255, 0), -1)  # Green dot
    cv2.circle(image, right_iris_px, 3, (0, 255, 0), -1)  # Green dot

    return gaze_x, gaze_y


def is_looking_at_screen(gaze_x, gaze_y):
    """
    Determine if the person is looking at the screen based on gaze direction
    """
    # Define thresholds for central gaze
    x_threshold = 0.14  # How far left or right
    y_threshold = 0.14 # How far up or down

    # Check if gaze is within thresholds
    return abs(gaze_x) < x_threshold and abs(gaze_y) < y_threshold


def are_eyes_open(left_ear, right_ear):
    """
    Determine if eyes are open based on eye aspect ratio
    """
    ear_threshold = 0.2  # Threshold for eye openness
    return left_ear > ear_threshold and right_ear > ear_threshold


# -----------------------------
# Step 4: Main detection function
# -----------------------------

def run_detection():
    # Initialize webcam
    cap = cv2.VideoCapture(0)

    # Check if camera opened successfully
    if not cap.isOpened():
        print("Error: Could not open camera.")
        return

    # Initialize tracking variables
    start_time = time.time()
    total_time = 0
    present_time = 0
    focused_time = 0

    # Initialize state machine counters
    presence_counter = 0
    absence_counter = 0
    focus_counter = 0
    distraction_counter = 0

    # Required consecutive frames to change state
    presence_threshold = 10
    focus_threshold = 10

    # Current states
    is_present = False
    is_focused = False
    last_arduino_state = None

    # Events tracking
    absence_events = []
    distraction_events = []
    current_absence = None
    current_distraction = None

    print("Starting detection. Press 'q' to quit.")

    try:
        while cap.isOpened():
            # Read frame from camera
            success, image = cap.read()
            if not success:
                print("Error: Failed to grab frame.")
                break

            # Update time metrics
            current_time = time.time()
            time_diff = current_time - start_time
            start_time = current_time
            total_time += time_diff

            # Convert to RGB (MediaPipe requires RGB input)
            image_rgb = cv2.cvtColor(image, cv2.COLOR_BGR2RGB)

            # Process the image with MediaPipe
            results = face_mesh.process(image_rgb)

            # Initialize detection flags
            face_detected = False
            focus_detected = False

            # Check if face landmarks are detected
            if results.multi_face_landmarks:
                face_detected = True
                face_landmarks = results.multi_face_landmarks[0]  # Get first face

                # Draw face mesh landmarks
                mp_drawing.draw_landmarks(
                    image=image,
                    landmark_list=face_landmarks,
                    connections=mp_face_mesh.FACEMESH_CONTOURS,
                    landmark_drawing_spec=None,
                    connection_drawing_spec=mp_drawing_styles.get_default_face_mesh_contours_style()
                )

                # Draw iris landmarks
                mp_drawing.draw_landmarks(
                    image=image,
                    landmark_list=face_landmarks,
                    connections=mp_face_mesh.FACEMESH_IRISES,
                    landmark_drawing_spec=None,
                    connection_drawing_spec=mp_drawing_styles.get_default_face_mesh_iris_connections_style()
                )

                # Calculate eye aspect ratio (openness)
                left_ear = calculate_eye_aspect_ratio(face_landmarks.landmark, LEFT_EYE)
                right_ear = calculate_eye_aspect_ratio(face_landmarks.landmark, RIGHT_EYE)
                avg_ear = (left_ear + right_ear) / 2

                # Analyze gaze direction
                gaze_x, gaze_y = analyze_gaze_direction(face_landmarks.landmark, image)

                # Check if eyes are open and looking at screen
                eyes_open = are_eyes_open(left_ear, right_ear)
                looking_at_screen = is_looking_at_screen(gaze_x, gaze_y)

                # Consider focused if eyes are open and looking at screen
                focus_detected = eyes_open and looking_at_screen

                # Display metrics on screen
                h, w, c = image.shape
                cv2.putText(image, f"EAR: {avg_ear:.2f}",
                            (10, 180), cv2.FONT_HERSHEY_SIMPLEX, 0.6, (255, 255, 255), 1)
                cv2.putText(image, f"Gaze X/Y: {gaze_x:.2f}/{gaze_y:.2f}",
                            (10, 210), cv2.FONT_HERSHEY_SIMPLEX, 0.6, (255, 255, 255), 1)

                # Display status indicators
                eye_color = (0, 255, 0) if eyes_open else (0, 0, 255)
                gaze_color = (0, 255, 0) if looking_at_screen else (0, 0, 255)

                cv2.putText(image, f"Eyes: {'Open' if eyes_open else 'Closed'}",
                            (w - 150, 180), cv2.FONT_HERSHEY_SIMPLEX, 0.6, eye_color, 1)
                cv2.putText(image, f"Gaze: {'Center' if looking_at_screen else 'Away'}",
                            (w - 150, 210), cv2.FONT_HERSHEY_SIMPLEX, 0.6, gaze_color, 1)

            # State machine for presence detection
            if face_detected:
                presence_counter += 1
                absence_counter = 0
                if presence_counter >= presence_threshold and not is_present:
                    is_present = True
                    if current_absence is not None:
                        absence_duration = current_time - current_absence
                        absence_events.append({
                            'start': datetime.fromtimestamp(current_absence).strftime('%H:%M:%S'),
                            'end': datetime.fromtimestamp(current_time).strftime('%H:%M:%S'),
                            'duration': absence_duration
                        })
                        current_absence = None
            else:
                absence_counter += 1
                presence_counter = 0
                if absence_counter >= presence_threshold and is_present:
                    is_present = False
                    current_absence = current_time

            # Update present time based on current state
            if is_present:
                present_time += time_diff

            # State machine for focus detection
            if focus_detected:
                focus_counter += 1
                distraction_counter = 0
                if focus_counter >= focus_threshold and not is_focused:
                    is_focused = True
                    if current_distraction is not None:
                        distraction_duration = current_time - current_distraction
                        distraction_events.append({
                            'start': datetime.fromtimestamp(current_distraction).strftime('%H:%M:%S'),
                            'end': datetime.fromtimestamp(current_time).strftime('%H:%M:%S'),
                            'duration': distraction_duration
                        })
                        current_distraction = None
            else:
                distraction_counter += 1
                focus_counter = 0
                if distraction_counter >= focus_threshold and is_focused:
                    is_focused = False
                    current_distraction = current_time

            # Update focused time based on current state
            if is_focused:
                focused_time += time_diff

            # Calculate focus scores
            presence_score = (present_time / total_time) * 100 if total_time > 0 else 0
            focus_score = (focused_time / present_time) * 100 if present_time > 0 else 0
            overall_score = (focused_time / total_time) * 100 if total_time > 0 else 0


            # Display main status
            presence_status = "Present" if is_present else "Absent"
            presence_color = (0, 255, 0) if is_present else (0, 0, 255)
            cv2.putText(image, f"Status: {presence_status}",
                        (10, 30), cv2.FONT_HERSHEY_SIMPLEX, 0.7, presence_color, 2)

            focus_status = "Focused" if is_focused else "Distracted"
            focus_color = (0, 255, 0) if is_focused else (0, 165, 255)
            cv2.putText(image, f"Focus: {focus_status}",
                        (10, 60), cv2.FONT_HERSHEY_SIMPLEX, 0.7, focus_color, 2)

            # Display scores
            cv2.putText(image, f"Presence: {presence_score:.1f}%",
                        (10, 90), cv2.FONT_HERSHEY_SIMPLEX, 0.7, (255, 255, 255), 2)
            cv2.putText(image, f"Focus Rate: {focus_score:.1f}%",
                        (10, 120), cv2.FONT_HERSHEY_SIMPLEX, 0.7, (255, 255, 255), 2)
            cv2.putText(image, f"Overall Focus: {overall_score:.1f}%",
                        (10, 150), cv2.FONT_HERSHEY_SIMPLEX, 0.7, (255, 255, 255), 2)

            # Display Arduino connection status
            arduino_status = "Connected" if arduino_connected else "Not Connected"
            arduino_color = (0, 255, 0) if arduino_connected else (0, 0, 255)
            cv2.putText(image, f"Arduino: {arduino_status}",
                        (10, 240), cv2.FONT_HERSHEY_SIMPLEX, 0.6, arduino_color, 1)


            # Send state to Arduino
            current_arduino_state = None
            if not is_present:
                current_arduino_state = 'A'
            elif is_focused:
                current_arduino_state = 'F'
            else:
                current_arduino_state = 'D'

            if current_arduino_state != last_arduino_state:
                send_to_arduino(current_arduino_state)
                last_arduino_state = current_arduino_state

            # Display the status indicator in top-right corner
            h, w, c = image.shape
            indicator_radius = 20
            margin = 30

            if not is_present:
                color = (0, 0, 255)  # Red for absent
                text = "ABSENT"
            elif is_focused:
                color = (0, 255, 0)  # Green for focused
                text = "FOCUSED"
            else:
                color = (0, 165, 255)  # Orange for distracted
                text = "DISTRACTED"

            # Draw indicator circle
            center = (w - margin, margin)
            cv2.circle(image, center, indicator_radius, color, -1)

            # Add text label
            cv2.putText(image, text,
                        (w - margin - len(text) * 5, margin + indicator_radius + 15),
                        cv2.FONT_HERSHEY_SIMPLEX, 0.5, color, 1)

            # Display the image
            cv2.imshow('Eye Direction and Presence Detection', image)

            # Exit on 'q' key press
            if cv2.waitKey(5) & 0xFF == ord('q'):
                break

    finally:

        # Close arduino
        if arduino_connected and arduino:
            arduino.close()
            print("Arduino connection closed")

        # Release resources
        cap.release()
        cv2.destroyAllWindows()

        # Print summary
        print("\n----- Session Summary -----")
        print(f"Total session time: {total_time:.1f} seconds")
        print(f"Present time: {present_time:.1f} seconds ({presence_score:.1f}%)")
        print(f"Focused time: {focused_time:.1f} seconds ({focus_score:.1f}%)")
        print(f"Overall focus score: {overall_score:.1f}%")
        print(f"Number of absence events: {len(absence_events)}")
        print(f"Number of distraction events: {len(distraction_events)}")

        if absence_events:
            print("\nAbsence Events:")
            for i, event in enumerate(absence_events, 1):
                print(f"  {i}. {event['start']} to {event['end']} ({event['duration']:.1f} seconds)")

        if distraction_events:
            print("\nDistraction Events:")
            for i, event in enumerate(distraction_events, 1):
                print(f"  {i}. {event['start']} to {event['end']} ({event['duration']:.1f} seconds)")


if __name__ == "__main__":
    run_detection()