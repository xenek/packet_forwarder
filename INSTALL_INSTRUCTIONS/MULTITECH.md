# Install the TTN Packet Forwarder on a Multitech Conduit

*Note: if you're using an AEP model, you will need to configure the Conduit on the web interface before installing the Packet Forwarder. Consult [this guide](https://www.thethingsnetwork.org/docs/gateways/multitech/aep.html) to learn how to do this.*

## Download and install

1. Download the [Multitech Conduit package](https://ttnreleases.blob.core.windows.net/packet_forwarder/master/multitech-conduit-pktfwd.zip) of the packet forwarder.

2. In the archive, you will find an `.ipk` file, as well as the executable binary. Unless you want to test the packet forwarder before installing it, you won't need to use the binary. Copy the `.ipk` file on the Multitech mConduit, using either a USB stick or through the network.

3. Install the package, configure the packet forwarder, then start it:

```bash
$ opkg install ttn-pkt-fwd.ipk
Installing ttn-pkt-fwd (2.0.0) to root...
Configuring ttn-pkt-fwd.
$ /etc/init.d/ttn-pkt-fwd configure
[...]
# Following the instructions of the wizard
  INFO New configuration file saved             ConfigFilePath=/var/config/ttn-pkt-fwd/config.yml
$ /etc/init.d/ttn-pkt-fwd start
Starting ttn-pkt-fwd: OK
```

## <a name="build"></a>Build the TTN Packet Forwarder for the Multitech Conduit

If you use a specific machine or want to contribute to the development of the packet forwarder, you might want to build the TTN Packet Forwarder. You might need to use a Linux environment to run the toolchain necessary for the build.

### Downloading the Multitech toolchain

To build the packet forwarder, you will need to download [Multitech's C toolchain](http://www.multitech.net/developer/software/mlinux/mlinux-software-development/mlinux-c-toolchain/). Download it, and install it by following the instructions on Multitech's website.

### Building the binary

1. Download the packet forwarder, along with its dependencies:

```bash
$ go get -u github.com/TheThingsNetwork/packet_forwarder
$ cd $GOPATH/src/github.com/TheThingsNetwork/packet_forwarder
$ make dev-deps
$ make deps
```

2. Enable the Multitech toolchain, then set those environment variables:

```bash
$ source /path/to/sdk/environment-setup-arm926ejste-mlinux-linux-gnueabi
# Usually /opt/mlinux/{version}/environment-setup-arm926ejste-mlinux-linux-gnueabi
$ export GOARM=5
$ export GOOS=linux
$ export GOARCH=arm
$ export CFG_SPI=ftdi
$ export PLATFORM=multitech
```

3. Build the binary:

```bash
$ make build
```

The binary will then be available in the `release/` folder.

### Building the package

To build the package, use the `create-kerlink-package.sh` script:

```bash
$ ./scripts/create-kerlink-package.sh release/packet-forwarder-linux-arm-multitech-ftdi
[...]
./scripts/create-kerlink-package.sh: package available at ttn-pkt-fwd-multitech.ipk
```

The package will then be available at the specified path.
