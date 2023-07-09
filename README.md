# go-mirror

I created this script in [GO](https://go.dev) while waiting for a sync of a large file tree.

The script mirrors a file tree to another location. File operations run in parallel. It was tested on Windows. No warranties, see [LICENSE](LICENSE). THIS IS NOT PRODUCTION GRADE SOFTWARE!

To build the binary, run
```shell
go install github.com/binChris/go-mirror
```

Execute with
```
go-mirror (flags) (source dir) (destination dir)
```

Run with `-help` to see available flags.
