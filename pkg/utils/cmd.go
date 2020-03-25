package utils

import (
    "os";
    "os/signal";
    "syscall";
)

func InterruptSignal() <-chan os.Signal {
    wait := make(chan os.Signal, 1)
    signal.Notify(wait, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
    return wait
}