# Vertex Studio

**Vertex Studio** is a production-grade CLI tool built in Go for generating cinematic AI videos using Google's **Vertex
AI** (specifically the **Veo 3.1** model).

It automates the entire pipeline: authenticating with Google Cloud, managing Long-Running Operations (LROs), downloading
high-fidelity video assets, and stitching them into a final movie using **FFmpeg**.

## 🚀 Features

- **Veo 3.1 Integration:** Uses the latest Google Veo model for 1080p/4K video generation.
- **Scene Continuity:** Automatically extracts the last frame of a generated video and uses it as the starting frame for
  the next segment (Image-to-Video). This ensures characters and scenes remain consistent across different shots.
- **Robust Error Handling:** Implements a custom REST client to handle Vertex AI's "Long Running Operations" (LRO) and
  polling logic, bypassing SDK beta instability.
- **Automated Stitching:** Uses FFmpeg to merge all generated segments into a seamless final movie (`final_movie.mp4`).
- **Secure Authentication:** Supports both `gcloud` CLI (ADC) and Service Account JSON keys.

---

## 🛠️ Prerequisites

Before running the project, ensure you have the following installed:

1. **Go** (1.25.5 or newer)
    - [Download Go](https://go.dev/dl/)
2. **FFmpeg** (Required for video stitching)
    - Mac: `brew install ffmpeg`
    - Linux: `sudo apt install ffmpeg`
    - Windows: `winget install ffmpeg`
3. **Google Cloud CLI (`gcloud`)**
    - [Install gcloud](https://cloud.google.com/sdk/docs/install)
    - Used for authentication token generation and high-speed downloads from Cloud Storage.

---

## ⚙️ Setup & Installation

1. **Clone the Repository**
   ```bash
   git clone [https://github.com/shamspias/vertex-studio.git](https://github.com/shamspias/vertex-studio.git)
   cd vertex-studio
   ```

2. **Initialize the Module**
   ```bash
   go mod tidy
   ```

3. **Configure Environment Variables**
   Create a `.env` file in the root directory:
   ```ini
   # .env
   
   # Your Google Cloud Project ID (Must have Vertex AI API enabled)
   GOOGLE_CLOUD_PROJECT=your-project-id
   
   # Region (e.g., us-central1)
   GOOGLE_CLOUD_LOCATION=us-central1
   
   # Optional: Path to Service Account JSON (if not using gcloud login)
   # GOOGLE_APPLICATION_CREDENTIALS=credentials.json
   
   # Output Directory
   OUTPUT_DIR=output
   ```

4. **Authentication**
   Authenticate your `gcloud` session so the tool can generate tokens:
   ```bash
   gcloud auth application-default login
   ```
   *(Alternatively, place your Service Account JSON key in the folder and set `GOOGLE_APPLICATION_CREDENTIALS` in
   the `.env` file)*.

---

## 📝 Configuration (Prompt Engineering)

The video generation is controlled by a JSON configuration file located at `config/prompt.config`.

**Example `config/prompt.config`:**

```json
{
  "global_settings": {
    "aspect_ratio": "16:9",
    "negative_prompt": "blurry, distortion, low quality, bad anatomy, text, watermark"
  },
  "segments": [
    {
      "duration": 8,
      "prompt": "Cinematic 8k close up. A desperate man in a green trench coat making a call on a rotary-style wall phone. Green neon lighting. Cyberpunk noir atmosphere."
    },
    {
      "duration": 8,
      "prompt": "Cinematic 8k. The man slams the phone down and looks over his shoulder in fear. Rain is falling in the background. Green neon reflections on his wet coat."
    }
  ]
}

```

* **Segments:** The tool processes these in order. Segment 2 will use the last frame of Segment 1 as a reference to keep
  visual consistency.

---

## ▶️ Usage

### **1. Run the Studio**

To generate the video pipeline:

```bash
make run

```

* This will compile the Go binary.
* It reads `config/prompt.config`.
* It calls Vertex AI for each segment.
* It stitches the results into `output/final_movie.mp4`.

### **2. Clean Workspace**

To remove generated videos and binaries:

```bash
make clean

```

## ⚠️ Troubleshooting

* **`403 Permission Denied`**: Ensure your Google Cloud Project has the **Vertex AI API** enabled and your user/service
  account has the **Vertex AI User** role.
* **`gcloud: command not found`**: Ensure the Google Cloud SDK is installed and in your system PATH.
* **`ffmpeg error`**: Ensure FFmpeg is installed and accessible from your terminal.
