// +build windows

package app

func notifyParentProcess() {
	appLog.Info("Windows does not have signals.")
}
