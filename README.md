# Stay FocuSD

Stay FocuSD is a cross platform productivity and study companion that combines a backend and frontend to help you maintain focus during study sessions. It features a smart LED indicator system powered by AI based attention tracking to provide real time feedback on your focus levels.

## Features

    Real-time Attention Tracking
    - Uses computer vision to detect face presence and attention levels
    - Provides immediate visual feedback through LED indicators

    Session Analytics
    - Tracks total session duration
    - Measures attention ratio
    - Counts distraction incidents
    - Calculates average distraction duration
    - Provides time-series analysis of focus patterns


## How It Works
Using the frontend to send request to our server. We monitor your focus levels during study sessions. The system utilizes a webcam to capture real-time video, which is processed using OpenCV to detect face presence and attention levels. Based on the analysis, it provides immediate feedback through an LED indicator system.
 **API Endpoints**
   - `/upload`: Handles webcam image processing
   - `/analyze`: Processes session data and generates analytics


## Tech Stack
    - Go (Gin web framework)
    - Python (OpenCV, ML processing)
    - RESTful API architecture

    Machine Learning
    - OpenCV
    - Haar Cascade Classifiers
    - Real-time image processing

## Getting Started

1. Set up the Arduino hardware according to the pin configuration:
   - Red LED: Pin 7
   - Yellow LED: Pin 10
   - Green LED: Pin 12

2. Install backend dependencies:
   ```bash
   go mod download
   ```

3. Install Python requirements:
   ```bash
   pip install opencv-python
   ```

4. Configure environment variables:
   - Create a `.env` file in the Backend directory
   - Set `PYTHON_PATH` if needed (defaults to "python3")

5. Start the backend server:
   ```bash
   go run main.go
   ```
## Data Privacy

Designed with privacy-first principles. Absolutely no user data is ever stored on any server or database.

    All analysis is done in real-time and discarded after processing.

    The only files saved are those the frontend explicitly writes, such as local session logs or exported summaries.

    No persistent storage, cloud sync, or user tracking is implemented.

This ensures complete control over your data and peace of mind during use.

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

## License
[MIT](https://choosealicense.com/licenses/mit/)
