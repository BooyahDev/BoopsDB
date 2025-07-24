#!/bin/bash

# Ensure the script is run as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root (using sudo)." >&2
    exit 1
fi

# Define variables for service files, binary files, and installation paths
SERVICE_URL="https://file.booyah.dev/BoopsDB-Client/"
BINARY_AMD64="${SERVICE_URL}boops_0.2_amd64.binary"
BINARY_ARM64="${SERVICE_URL}boops_0.2_arm64.binary"
BOOPS_BIN="/usr/local/bin/boops"

# Function to download a file from URL with error handling
download_file() {
    local url=$1
    local destination=$2

    echo "Downloading $url to $destination..."
    curl -fsSL "$url" > "$destination"
    if [ $? -ne 0 ]; then
        echo "Failed to download $url. Please check your internet connection." >&2
        exit 1
    fi
}

# Download and configure systemd service and timer files
echo "Setting up systemd service and timer..."
download_file "${SERVICE_URL}boops.service" "/etc/systemd/system/boops.service"
download_file "${SERVICE_URL}boops.timer" "/etc/systemd/system/boops.timer"

# Reload systemd daemon, enable and start the timer
systemctl daemon-reload
if [ $? -ne 0 ]; then
    echo "Failed to reload systemd daemon." >&2
    exit 1
fi

systemctl enable boops.timer
if [ $? -ne 0 ]; then
    echo "Failed to enable boops timer." >&2
    exit 1
fi

systemctl start boops.timer
if [ $? -ne 0 ]; then
    echo "Failed to start boops timer. The service may still be enabled but not running." >&2
    exit 1
fi

# Download and install the binary files based on architecture
echo "Installing binary files..."
case "$(uname -m)" in
    x86_64)
        download_file "$BINARY_AMD64" "$BOOPS_BIN"
        ;;
    aarch64)
        download_file "$BINARY_ARM64" "$BOOPS_BIN"
        ;;
    *)
        echo "Unsupported architecture: $(uname -m)" >&2
        exit 1
        ;;
esac

# Make the binary executable
chmod +x "$BOOPS_BIN"

# Create the configuration directory if it doesn't exist
echo "Creating configuration directory..."
mkdir -p /etc/boops
if [ $? -ne 0 ]; then
    echo "Failed to create configuration directory." >&2
    exit 1
fi

# Interactive machine registration
read -rp "Do you want to register a new machine now? (y/n): " REGISTER_MACHINE
if [[ "$REGISTER_MACHINE" == [Yy] ]]; then
    read -rp "Please enter the Machine ID: " MACHINE_ID
    echo "Registering machine with ID: $MACHINE_ID..."
    "$BOOPS_BIN" regist "$MACHINE_ID"
    if [ $? -ne 0 ]; then
        echo "Machine registration failed." >&2
        exit 1
    else
        echo "Machine registered successfully."
    fi
else
    echo "Skipping machine registration."
fi

echo "Installation completed successfully!"
exit 0
