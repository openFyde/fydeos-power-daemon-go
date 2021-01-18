package main

import (
  "fmt"
  "os"
  "os/exec"
  "context"
  "runtime"
  "time"
  "github.com/godbus/dbus"
  "fydeos.com/power_daemon/dbusutil"
  "fydeos.com/power_daemon/suspend_manager"
)

// Debug related begin
const debug = true

func trace() string{
    pc, _, _, ok := runtime.Caller(2)
    if !ok { return "?"}

    fn := runtime.FuncForPC(pc)
    return fn.Name()
}

func dPrintln(a ...interface{}) {
  if debug {
    fmt.Println(time.Now().Local(), trace(), a...)
  }
}
//Debug related end

main() {
  conn, err := dbus.ConnectSystemBus()
  if err != nil {
    dPrintln("Connect system bus error:%w", err)
    os.Exit(1)
  }
  defer conn.Close()
  ctx,cancel := context.WithCancel(context.Background())
  defer cancel()
  sigServer := dbusutil.NewSignalServer(ctx, conn)
  suspendManager := suspend_manager.NewSuspendManager(ctx, conn)
  suspendManager.Register(sigServer)
  defer suspendManager.UnRegister()
  sigServer.StartWorking()
}
