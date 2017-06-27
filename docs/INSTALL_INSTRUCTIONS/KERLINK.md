
# Install the TTN Packet Forwarder on a Kerlink IoT Station

*Note: for the moment, the TTN Packet Forwarder is not compatible with the Kerlink iBTS.*

Key observations.

- Always shut down the Kerlink properly. While it has a battery and shuts down automatically when power is removed, the nandflash MTD device file system can corrupt if the internal battery is discharged after multiple long power outages with very short uptimes, as the Kerlink linux build might not have enough power for a proper shutdown. The suggested technique is to issue a shutdown now command from the terminal or through SSH.

- Updating to v3.1 from 2.3.3 of the Kerlink System Firmware is a one-way process. Kerlink, on their wiki, state that attempting a downgrade to v2.3.3 after installing v3.1 can brick the device and also say "Firmware wirnet_v3.1 is mandatory to increase nandflash robustness. Once installed, older firmwares cannot be installed anymore (breaking of backward compatibility)." The key advantage of upgrading to v3.1 is use of a better flash file system - transitioning from yaffs2 to ubifs.

- When running the packet forwarder temporarily or manually you typically start it running on the 8GB eMMC. This is not suitable for any mid to long term use, especially if logging or freqently writing. The recommended way to run the packet forwarder is to permanently install it to the 128MB nand flash, in the user space called /mnt/fsuser-1. The easiest way to achieve this is through creating a DOTA file using the provided TTN script, placing it in the DOTA directory using scp, and rebooting, allowing the Kerlink System Firmware to decompress, install and automatically configure it to run in the background at startup.

- Copying a damaged DOTA file using scp to the DOTA directory for permanent installation can prevent the device operating properly. The Kerlink wiki details how to recover in this instance, however our best suggestion is to use ls -l to check the DOTA file size prior to copying it.   

Prior to installing the TTN Packet Forwarder, we recommend **updating the Station to the latest firmware available**.
Kerlink 

+ [Download and test the TTN Packet Forwarder](#download-test)
+ [Install the TTN Packet Forwarder](#install)
+ [Build the TTN Packet Forwarder](#build)
+ [Troubleshooting](#troubleshooting)

## <a name="download-test"></a>Download and test the TTN Packet Forwarder

*Note: Before installing the new packet forwarder, make sure you removed any other packet forwarder installed on your Kerlink IoT Station. If you don't have any important files stored on the disk, the safest way to make sure of that is to update the Station to the latest firmware available, which will reset the file system in the process. Note that a restore through pressing reset 22 times does not reset the userfs-1 file system, unlike a install from USB of the latest firmware.*

1. Download the [Kerlink build](https://ttnreleases.blob.core.windows.net/packet-forwarder/master/kerlink-iot-station-pktfwd.tar.gz) of the packet forwarder.

2. In the folder, you will find several files: a `create-package.sh` script and a binary file, that we will call `packet-forwarder`.

The binary is sufficient for a testing use - if you wish to try the TTN packet forwarder, just copy the binary on the Station, and execute:

```bash
$ ./packet-forwarder configure
# Follow the instructions of the wizard
[...]
  INFO New configuration file saved             ConfigFilePath=/root/config.yml
$ ./packet-forwarder start
  INFO Packet Forwarder for LoRa Gateway        HALVersionInfo=Version: 4.0.0; Options: native;
[...]
  INFO Concentrator started, packets can now be received and sent
```

## <a name="install"></a>Install the TTN Packet Forwarder

This section covers permanent installation of the TTN Packet Forwarder on a Kerlink IoT Station.

### Packaging the TTN Packet Forwarder into a DOTA file for permanent installation

Download the [Kerlink build](https://ttnreleases.blob.core.windows.net/packet-forwarder/master/kerlink-iot-station-pktfwd.tar.gz) of the packet forwarder. Execute the `create-package.sh` script with the binary inside as an argument:

```bash
$ ./create-package.sh packet-forwarder
[...]
# The script will ask you several questions to configure the packet forwarder.
./create-package.sh: Kerlink DOTA package complete.
```

A `kerlink-release-<id>` folder will appear in the folder you are in:

```bash
$ cd kerlink-release-<id> && tree
.
├── dota_ttn-pkt-fwd.tar.gz
├── INSTALL.md
└── produsb.sh
```

### Transfering and installing the TTN Packet Forwarder

You can consult the `INSTALL.md` to know how to install the package from here. The two options are **network transfer** and **USB stick transfer**. Depending on your configuration, choose the installation method that suits you best. Once the package has been transferred, the packet forwarder will be installed on your Kerlink IoT Station! You can monitor it from the [console](https://console.thethingsnetwork.org):

![Console demo](https://github.com/TheThingsNetwork/packet_forwarder/raw/master/docs/INSTALL_INSTRUCTIONS/console.gif)

## <a name="build"></a>Build the TTN Packet Forwarder for the Kerlink IoT Station

If you use a specific machine or want to contribute to the development of the packet forwarder, you might want to build the TTN Packet Forwarder. You might need to use a Linux environment to run the toolchain necessary for the build.

### Building the binary

To build the packet forwarder for the Kerlink IoT Station, you will need access to Kerlink's Wirnet Station wiki.

1. On Kerlink's Wirnet Station Wiki, click on *Resources*, scroll down to *Tools*, then download the toolchain you need, depending on the firmware of your Station.

2. In most cases, the archive will hold a `arm-2011.03-wirgrid` folder. Copy this folder in `/opt`:

```bash
$ mkdir -p /opt
$ mv arm-2011.03-wirgrid /opt
```

3. Execute the `scripts/build-kerlink.sh` script, indicating in argument the location of the toolchain:

```bash
$ ./scripts/build-kerlink.sh "/opt/arm-2011.03-wirgrid"
```

This script will build the Kerlink IoT Station binary of the packet forwarder.

### Building the DOTA file

This binary is sufficient for basic testing of the packet forwarder on a Kerlink IoT Station. However, for permanent installations, the packet forwarder is wrapped in a package called DOTA file. To create a DOTA file, use the `scripts/kerlink/create-kerlink-package.sh` script:

```bash
$ ./scripts/kerlink/create-kerlink-package.sh <packet-forwarder-binary-path>
[...]
create-kerlink-package.sh: Kerlink DOTA package complete. The package is available in kerlink-release/. Consult the INSTALL.md file to know how to install the package on your Kerlink IoT Station!
```

## <a name="troubleshooting"></a>Troubleshooting

#### I've deleted my gateway from the console and added a new one, how can I change the configuration of the gateway?

Connect remotely to the Kerlink IoT Station, and execute these commands:

```bash
$ cd /mnt/fsuser-1/ttn-pkt-fwd
$ ./ttn-pkt-fwd configure config.yml
# Following the instructions of the wizard
[...]
  INFO New configuration file saved             ConfigFilePath=config.yml
$ reboot
# Reboot the gateway to apply the changes
```

#### I'm getting a "Concentrator boot time computation error: Absurd uptime received by the concentrator" error when starting the packet forwarder.

The concentrator sometimes sends absurd uptime values to the packet forwarder, often because it hasn't been stopped properly. Restart the packet forwarder until this error disappears.

#### Running the binary (not the DOTA) manually gives an error "x509: failed to load system roots and no roots provided"

The packet forwarder when manually run needs a current SSL certificate. Use cd /etc/ssl/certs to enter the correct directory. Use wget https://letsencrypt.org/certs/lets-encrypt-x3-cross-signed.pem.txt to fetch the certificate.

#### I can't use wget to fetch any files directly from the internet on the Kerlink

Use vi to edit /etc/sysconfig/network and disable the firewall. Also, consider changing the DNS servers. Google's are 8.8.8.8 and 8.8.4.4. A restart through the reboot command is required after these changes.
