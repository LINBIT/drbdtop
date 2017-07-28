# drbdtop
Like top, but DRBD

drbdtop simplifies monitoring and administrating for DRBD resources. Previously
available tools and techniques that provide status information on DRBD do not
allow you to change the status of DRBD resources, and tools that allow you to
change the status of DRBD resources, do not provide information on their status.

In addition to allowing the user control and monitor the general state of
resources from the same utility, detailed status information is also provided to
help the user gain insight to performance-related factors.

The best sources of information on drbdtop are our
[gh pages site](https://linbit.github.io/drbdtop/) and running `drbdtop --help`.

## Usage

### Interactive Mode
To start the interactive TUI simply run the `drbdtop` command.

For a short introduction to this view, please read this
[short article](https://linbit.github.io/drbdtop/guides/intro/).

## Building and Installing
drbdtop is written in Go. If you haven't built a Go program before, please refer
to this [helpful guide](https://golang.org/doc/install).

```bash
go github.com/linbit/drbdtop
```

## Contributing
We welcome anyone to contribute, but we'd like you to please file an issue
to get the conversation started before making a pull request. Development
happens on the `develop` branch, which can be rebased every time. The `master`
branch is reserved for releases.

## License
GPL2
