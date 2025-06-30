# Build stage for Go application
FROM golang:1.24 AS builder
WORKDIR /app
# Copy the entire backend directory first
COPY . .
# Then build the application
RUN CGO_ENABLED=0 go build -o app .

# Final stage with Python
FROM python:3.9-slim
WORKDIR /app

# Copy built Go application and model files
COPY --from=builder /app/app .
COPY --from=builder /app/model ./model

# Install OpenCV dependencies
RUN apt-get update && apt-get install -y \
    libgl1 \
    libglib2.0-0 \
    && rm -rf /var/lib/apt/lists/*

# Install Python packages
RUN pip install opencv-python numpy

# Create uploads directory
RUN mkdir -p uploads && chmod 755 uploads

# Set environment variables
ENV PYTHON_PATH=/usr/local/bin/python
ENV PORT=8080

# Expose port
EXPOSE 8080

# Run the application
CMD ["./app"]