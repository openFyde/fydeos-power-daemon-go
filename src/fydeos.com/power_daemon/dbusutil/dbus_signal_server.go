package dbusutil

import (
  "context"
  "time"
  "fmt"
  "github.com/godbus/dbus"
)

const (
  dbusInterface = "org.chromium.PowerManager"
  dbusPath = "/org/chromium/PowerManager"
  dbusSender = "org.chromium.PowerManager"
  debug = true
)

type SignalHandler func(*dbus.Signal) error

type SignalHandlers []SignalHandler

type SignalMap map[string]SignalHandlers

type SignalServer struct {
  ctx context.Context
  conn *dbus.Conn
  sigmap SignalMap
}

func dPrintln(format string, a ...interface{}) {
  if debug {
    fmt.Printf("%s: ",time.Now().Local())
    fmt.Printf(format, a...)
    fmt.Println("")
  }
}

func NewSignalServer(ctx context.Context, conn *dbus.Conn) *SignalServer {
  return &SignalServer{ctx, conn, make(SignalMap)}
}

func (sigServer *SignalServer) RegisterSignalHandler(sigName string, handler SignalHandler) {
  handlers, ok := sigServer.sigmap[sigName]
  if !ok {
    handlers = make(SignalHandlers,2)
    sigServer.sigmap[sigName] = handlers
  }
  /*
  for _, h := range handlers {
    if h == handler {
      return
    }
  }
  */
  handlers = append(handlers, handler)
}
/*
func (sigServer *SignalServer) RevokeSignalHandler(sigName string, handler SignalHandler) {
  handlers, ok := sigServer.sigmap[sigName]
  if !ok {
    return
  }
  for _, h := range handlers {
    if h == handler {
      h = nil  // we never reduce slice
      return
    }
  }
}
*/
func (sigServer *SignalServer) addMatchSignal(sigName string) error {
  dPrintln("Add signal filter path:%s, interface:%s, signal:%s",
    dbusPath, dbusInterface, sigName)
  return sigServer.conn.AddMatchSignal(dbus.WithMatchObjectPath(dbusPath),
    dbus.WithMatchInterface(dbusInterface),
    dbus.WithMatchMember(sigName))
}

func (sigServer *SignalServer) removeMatchSignal(sigName string) error {
  dPrintln("Remove signal filter path:%s, interface:%s, signal:%s",
    dbusPath, dbusInterface, sigName)
  return sigServer.conn.RemoveMatchSignal(dbus.WithMatchObjectPath(dbusPath),
      dbus.WithMatchInterface(dbusInterface),
          dbus.WithMatchMember(sigName))
}

func (sigServer *SignalServer) addAllSignals() {
  for name, _ := range sigServer.sigmap {
     if err := sigServer.addMatchSignal(name); err != nil {
       dPrintln("Add signal %s, got error: %w", name, err)
     }
  }
  dPrintln("Finnished add signal filters.")
}

func (sigServer *SignalServer) removeAllSignals() {
  for name, _ := range sigServer.sigmap {
     if err := sigServer.removeMatchSignal(name); err != nil {
       dPrintln("Remove signal %s, got error: %w", name, err)
     }
  }
}

func (sigServer *SignalServer) handleSignal(sig *dbus.Signal) {
  member := sig.Name[len(dbusInterface)+1:]
  dPrintln("Get Signal %s, member: %s", sig.Name, member);
  if handlers, ok := sigServer.sigmap[member]; ok {
    for _, h := range handlers {
      if h != nil {
        if err := h(sig); err != nil {
          dPrintln("handler signal error:%w", err);
        }
      }
    }
  }
}

func (sigServer *SignalServer) StartWorking() {
  sigServer.addAllSignals()
  defer sigServer.removeAllSignals()
  ch := make(chan *dbus.Signal, 10)
  defer close(ch)
  sigServer.conn.Signal(ch)
  defer sigServer.conn.RemoveSignal(ch)
  dPrintln("Start listening signal...");
  for {
    select {
      case sig := <-ch:
        sigServer.handleSignal(sig)
      case <-sigServer.ctx.Done():
        return
    }
  }
}
