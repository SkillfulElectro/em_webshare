# em_webshare
[![Latest Release](https://img.shields.io/github/v/release/SkillfulElectro/em_webshare)](https://github.com/SkillfulElectro/em_webshare/releases)

- Simple and easy to use web based sharing file app

## Goal
- install an App on one device , share to all of the devices

## Features
- **Cross-platform**: Works on Windows, Linux, macOS, and Android.
- **Web-based UI**: Easy for client users to upload and download files via browser.
- **CLI Functionality on Android**: The Android app includes a console to run server commands.
- **Automated Builds**: Binaries for all platforms are automatically built and released.

## How to use
### Server Side
#### Desktop (CLI)
1. Download the `em_webshare` binary for your platform from the [latest release](https://github.com/SkillfulElectro/em_webshare/releases).
2. Start it using:
```sh
./em_webshare
```
It will host a web server on the first available port. Check your IPv4 address (e.g., using `ipconfig` on Windows or `ifconfig`/`ip addr` on Linux).

#### Android
1. Download and install the APK from the [latest release](https://github.com/SkillfulElectro/em_webshare/releases).
2. Open the app. It will start the server and show you the local IP and port.
3. Use the input field at the bottom to enter commands.

### Commands
- `upload <path>`: Add a file or directory to the download queue for clients. If the path is relative, it searches in the current working directory or the system Download folder.
- `ls <path>`: List files in a directory (defaults to current working directory).
- `cd <path>`: Change the current working directory for the CLI.
- `pwd`: Print the current working directory.
- `up_dir <path>`: Set the directory where files uploaded by clients will be saved.
- `exit`: Stop the server and exit.

You can upload multiple files and directories; they will be served in the order they were added (First Added, First Downloaded).

### Uploaded Files
By default, files uploaded by clients are saved in the `Downloads/em_webshare` folder on your system (on Android, this is `/sdcard/Download/em_webshare`). This makes it easy to find them in your file manager.

### Client Side
1. Open your browser and navigate to `http://<server-ip>:<port>`.
2. To **send** files: Choose files or a folder and press the "Send" button.
3. To **download** files: Press the "Download" button to get the next file/directory in the server's queue.
   - *Note*: If the server shares a directory, it will be streamed as a `.tar` file.

**⚠️ Warning: Ensure your OS firewall or Android permissions are not blocking the app. ⚠️**

## Contribution
Contribute at: https://github.com/SkillfulElectro/em_webshare.git
