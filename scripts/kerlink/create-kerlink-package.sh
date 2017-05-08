#!/bin/bash

# What this script does:
# - Downloads Let's Encrypt cert
# - Creates a configuration file
# - Generates a script and a readme file
# - Packages everything with the kerlink build for DOTA install
# Possible improvements:
# - Custom certificate address
# - TTN-hosted static url for produsb.sh and the Let's Encrypt cert

RED='\033[0;31m'
NC='\033[0m' # No Color

usage_str="usage: create-kerlink-package.sh [path to the Kerlink build]

example: ./create-kerlink-package.sh packet-forwarder-kerlink"

# Getting path to the kerlink binary
BINARY_PATH="$1"
if [[ -z "$BINARY_PATH" ]] ; then
    echo "$0: $usage_str"
    exit 1
fi

WORKDIR="/tmp/packet-forwarder-kerlink"
BASE="/mnt/fsuser-1/ttn-pkt-fwd"
CFG_FILENAME="config.yml"
PKTFWD_DESTDIR="$WORKDIR$BASE"

mkdir -p "$PKTFWD_DESTDIR"

cp "$BINARY_PATH" "$PKTFWD_DESTDIR/ttn-pkt-fwd"

configure () {
    DESTADDRESS="$1"

    printf "%s: Gateway ID:\n> " "$0"

    read -r gatewayID

    printf "%s: Gateway Key:\n> " "$0"

    read -r -s gatewayKey

    echo "id: \"${gatewayID}\"
key: \"${gatewayKey}\"" > "$DESTADDRESS"
}

echo "$0: If you haven't registered your gateway yet, register it on the console or with \`ttnctl\`, using the gateway connector protocol."

if [[ -f "$HOME/.pktfwd.yml" ]] ; then
    while true; do 
        read -r -p "$0: Local packet forwarder configuration found (in $HOME/.pktfwd.yml). Do you want to include it in the package? " yn
        case $yn in
            [Yy]* ) cp "$HOME/.pktfwd.yml" "$PKTFWD_DESTDIR/$CFG_FILENAME"; echo "$0: Local configuration included."; break;;
            [Nn]* ) echo "$0: Local packet forwarder configuration not copied, please enter the new configuration."; configure "$PKTFWD_DESTDIR/$CFG_FILENAME"; break;;
            * ) echo "Please answer [y]es or [n]o.";;
        esac
    done
else
    configure "$PKTFWD_DESTDIR/$CFG_FILENAME"
fi

echo "$0: Configuration saved - see INSTALL.md if you wish to modify this configuration later"

echo "$0: Fetching TLS root certificate"
SSL_WORKDIR="$WORKDIR/etc/ssl/certs"
mkdir -p "$SSL_WORKDIR"
pushd "$SSL_WORKDIR" &> /dev/null
wget "https://letsencrypt.org/certs/lets-encrypt-x3-cross-signed.pem.txt" &> /dev/null
popd &> /dev/null

echo "$0: Generating startup script"
echo "#!/bin/sh

BASE=\"$BASE\"
cd \$BASE
killall ttn-pkt-fwd
modem_off.sh

sleep 3
modem_on.sh
sleep 3
export GOGC=30
./ttn-pkt-fwd start --config=config.yml" > "$PKTFWD_DESTDIR/ttn-pkt-fwd.sh"
chmod +x "$PKTFWD_DESTDIR/ttn-pkt-fwd.sh"

echo "$0: Generating DOTA manifest"
echo "<?xml version=\"1.0\"?>
<manifest>
<app name=\"ttn-pkt-fwd\" appid=\"1\" shell=\"ttn-pkt-fwd.sh\">
<start autostart=\"y\" />
<stop kill=\"9\" />
</app>
</manifest>" > "$PKTFWD_DESTDIR/manifest.xml"

echo "$0: Startup and init scripts, build and manifests saved. Starting packaging"

mkdir kerlink-release

DOTA_ARCHIVE="dota_ttn-pkt-fwd.tar.gz"
pushd "$WORKDIR" &> /dev/null
tar -cvzf "$DOTA_ARCHIVE" "mnt" "etc" &> /dev/null
popd &> /dev/null
mv "$WORKDIR/$DOTA_ARCHIVE" kerlink-release

wget "https://cdn.rawgit.com/TheThingsNetwork/kerlink-station-firmware/16f6325e/dota/produsb.zip" &> /dev/null
unzip produsb.zip &> /dev/null # Creates a produsb.sh
mv produsb.sh kerlink-release
rm produsb.zip

echo "# Install the TTN Packet Forwarder on a Kerlink IoT Station

The Kerlink IoT Station build of the TTN packet forwarder is packaged within an archive, also called **DOTA file**.

## Method 1: USB stick

1. Copy \`$DOTA_ARCHIVE\` on an empty FAT or FAT32-formatted USB stick.
2. Copy \`produsb.sh\` on the USB stick.
3. Insert the stick in the Kerlink's USB port. Do not reboot the machine until the DOTA installation is complete! You can see the progress by pushing the \"Test\" button on the Station - as long as MOD1 and MOD2 are blinking, installation is in progress. It should take between 2 and 5 minutes.

## Method 2: Network transfer

1. Copy \`$DOTA_ARCHIVE\` in the \`/mnt/fsuser-1/dota\` folder on the Station, using \`scp\`.
2. Reboot the Station with \`reboot\` to trigger the DOTA installation. Do not try to shutdown the machine until the DOTA installation is complete! You can see the progress by pushing the \"Test\" button on the Station - as long as MOD1 and MOD2 are blinking, installation is in progress. It should take between 2 and 5 minutes." > kerlink-release/INSTALL.md

rm -rf "$WORKDIR"

printf "%s: ${RED}Kerlink DOTA package ready.${NC} The package is available in %s/kerlink-release. Consult the INSTALL.md file to know how to install the package on your Kerlink IoT Station!\n" "$0" "$PWD"
