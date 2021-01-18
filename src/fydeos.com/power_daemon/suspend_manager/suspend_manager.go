package suspend_manager

import (
  "strings"
  "context"
  "os"
  "os/exec"
  "time"
  "runtime"
  "fmt"
  "github.com/godbus/dbus"
  pmpb "chromiumos/system_api/power_manager_proto"
  "fydeos.com/power_daemon/dbusutil"
)

// Debug related begin
const debug = false

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

const {
  dbusInterface = "org.chromium.PowerManager"
  sigSuspendImminent = "SuspendImminent"
  sigSuspendDone = "SuspendDone"
  methdRegisterSuspendDelay = ".RegisterSuspendDelay"
  methdUnregisterSuspendDelay = ".UnregisterSuspendDelay"
  methdHandleSuspendReadiness = ".HandleSuspendReadiness"
  pathPreSuspendScript = "/etc/powerd/pre_suspend.sh"
  pathPostResumeScript = "/etc/powerd/post_resume.sh"
  serverDescription = "FydeOS Suspend Manager"
  execTimeout = 200
}

type SuspendManager struct {
  ctx *context.Context
  conn *dbus.Conn
  delay_id int
  suspend_id int
  on_suspend_delay bool
}

func NewSuspendManager(ctx context.Context, conn *dbus.Conn) (*SuspendManager, error) {
  return &SuspendManager{ctx, conn, 0, 0, false}
}

func (manager *SuspendManager) sendSuspendReadiness() error{
  req := &pmpb.SuspendReadinessInfo{manager.delay_id, manager.suspend_id}
  return dbusutil.CallProtoMethod(ctx, manager.conn.BusObject(), dbusInterface + methdHandleSuspendReadiness, req, nil)
}

func (manager *SuspendManager) handleSuspend(signal *dbus.Signal) error {
  if manager.on_suspend_delay {
    return errors.New("System is on suspend already")
  }
  suspendInfo := &pmpb.SuspendImminent{}
  if err := dbusutil.DecodeSignal(signal, suspendInfo); err != nil {
    return err
  }
  manager.suspend_id = suspendInfo.suspend_id
  manager.on_suspend_delay = true
  dPrintln("On suspend: %d, for reason %d", manager.suspend_id, suspendInfo.reason)
  if fi, err : = os.Stat(pathPreSuspendScript); err != nil {
    dPrintln("The script:%s is not exist.", pathPreSuspendScript)
  }
  ctx, cancel := context.WithTimeout(context.Background(), execTimeout * time.Millisecond)
  defer cancel()
  defer manager.sendSuspendReadiness()
  if err := exec.CommandContext(ctx, pathPreSuspendScript).Run(); err != nil {
    dPrintln("Exec pre-suspend script error:%w", err)
  }
  return nil
}

func (manager *SuspendManager) handleResume(signal *dbus.Signal) error {
  if !manager.on_suspend_delay {
    return errors.New("System is not on suspend")
  }
  suspendInfo := &pmpb.SuspendDone{}
  if err := dbusutil.DecodeSignal(signal, suspendInfo); err != nil {
    return err
  }
  if suspendInfo.suspend_id != manager.suspend_id {
    dPrintln("The resume suspend id is different from original")
  }
  manager.suspend_id = 0
  manager.on_suspend_delay = false
  dPrintln("On suspend: %d", manager.suspend_id)
  if fi, err : = os.Stat(pathPreSuspendScript); err != nil {
    dPrintln("The script:%s is not exist.", pathPreSuspendScript)
  }
  ctx, cancel := context.WithTimeout(context.Background(), execTimeout * time.Millisecond * 10)
  defer cancel()
  if err := exec.CommandContext(ctx, pathPostResumeScript).Run(); err != nil {
    dPrintln("Exec post-resume script error:%w", err)
  }
  return nil
}

func (manager *SuspendManager) Register(sigServer *SignalServer) error {
  req := &pmpb.RegisterSuspendDelayRequest{execTimeout, serverDescription}
  rsp := &pmpb.RegisterSuspendDelayReply{}
  err := dbusutil.CallProtoMethod(ctx, manager.conn.BusObject(), dbusInterface + methdRegisterSuspendDelay, req, rsp)
  if err != nil {
    return err
  }
  manager.delay_id = rsp.delay_id;
  sigServer.RegisterSignalHandler(sigSuspendImminent, func(sig *dbus.Signal){
    return manager.handleSuspend(sig)
  })
  sigServer.RegisterSignalHandler(sigSuspendDone, func(sig *dbus.Signal){
    return manager.handleResume(sig)
  })
}

func (manager *SuspendManager) UnRegister(sigServer *SignalServer) error {
  if manager.delay_id {
    req := &pmpb.UnregisterSuspendDelayRequest{manager.delay_id}
    return dbusutil.CallProtoMethod(ctx, manager.conn.BusObject(), dbusInterface + methdUnregisterSuspendDelay. req, nil)
  }
  return nil
}
