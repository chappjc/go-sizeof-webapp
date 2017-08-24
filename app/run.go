package app

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/chappjc/go-sizeof-webapp/internal/log"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func Run() (exitCode int) {
	var err error
	appLog, err = log.NewApplicationLogger()
	if err != nil {
		log.StdErr("could not create access log, reason -> %s", err.Error())
		return 1
	}

	if err = prepareTemplates(); err != nil {
		log.StdErr("could not parse html templates, reason -> %s", err.Error())
		return 1
	}

	httpPort := os.Getenv("_GO_HTTP")
	if httpPort == "" {
		httpPort = DefaultHttpPort
	}

	bindHttpHandlers()
	canExit, httpErr := make(chan sig, 1), make(chan error, 1)
	go func() {
		defer close(canExit)
		if err = http.ListenAndServe(httpPort, nil); err != nil {
			httpErr <- fmt.Errorf(
				"creating HTTP server on port '%s' FAILED, reason -> %s",
				httpPort, err.Error(),
			)
		}
	}()
	select {
	case err = <-httpErr:
		_ = appLog.Error(err.Error())
		log.StdErr(err.Error())
		return 1
	case <-time.After(300 * time.Millisecond):
	}

	notifyParentProcess()

	<-canExit
	return
}
