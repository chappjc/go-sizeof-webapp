Golang `sizeof` tips
------------------

**Web tool for interactive playing with Golang struct sizes.**  Made by [Kai Ren](https://github.com/tyranron) for Gopher Gala 2015.

## Aim
Provide comfortable tool to see how fields in struct are aligned,
to compare different structs and as the result - to understand
and remember alignment rules.

## Installing
Use [`dep`](https://github.com/golang/dep) to install dependencies
```bash
git clone https://github.com/chappjc/go-sizeof-webapp $GOPATH/src/chappjc/go-sizeof-webapp
cd $GOPATH/src/chappjc/go-sizeof-webapp
go get -u github.com/golang/dep/cmd/dep
$GOPATH/bin/dep ensure
go build # or go install
```

## Usage
```bash
./server -http=:7777 start
./server stop
./server restart
```

## Platform support
Tested on Linux and OS X x64 platforms, but should work properly and on other
*nix-like platforms. Daemonization is disabled on Windows, but it works.

## License
[Apache License 2.0](LICENSE)
