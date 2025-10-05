// +build windows

package service

import (
	"fmt"
	"log"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

const serviceName = "RemoteDesktopAgent"
const serviceDisplayName = "Remote Desktop Agent"
const serviceDescription = "Remote desktop access with lock screen support"

type Service struct {
	onStart func() error
	onStop  func() error
}

func NewService(onStart, onStop func() error) *Service {
	return &Service{
		onStart: onStart,
		onStop:  onStop,
	}
}

func (s *Service) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	
	// Start the service
	if err := s.onStart(); err != nil {
		log.Printf("Service start failed: %v", err)
		return true, 1
	}
	
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	log.Println("Service started successfully")

loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				log.Println("Service stopping...")
				break loop
			default:
				log.Printf("Unexpected control request #%d", c)
			}
		}
	}

	changes <- svc.Status{State: svc.StopPending}
	
	// Stop the service
	if err := s.onStop(); err != nil {
		log.Printf("Service stop failed: %v", err)
	}
	
	return
}

func RunService() error {
	isInteractive, err := svc.IsAnInteractiveSession()
	if err != nil {
		return fmt.Errorf("failed to determine if we are running in an interactive session: %w", err)
	}

	if !isInteractive {
		runService(false)
		return nil
	}

	// Running interactively (not as service)
	return nil
}

func runService(isDebug bool) {
	var err error
	if isDebug {
		elog = debug.New(serviceName)
	} else {
		elog, err = eventlog.Open(serviceName)
		if err != nil {
			return
		}
	}
	defer elog.Close()

	elog.Info(1, fmt.Sprintf("Starting %s service", serviceName))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	
	// This will be called from main.go with actual start/stop functions
	err = run(serviceName, &Service{})
	if err != nil {
		elog.Error(1, fmt.Sprintf("%s service failed: %v", serviceName, err))
		return
	}
	elog.Info(1, fmt.Sprintf("%s service stopped", serviceName))
}

var elog debug.Log

func InstallService(exePath string) error {
	// Service installation will be handled by external tools (sc.exe)
	// This function provides installation logic if needed
	return fmt.Errorf("use install-service.bat script to install the service")
}

func UninstallService() error {
	// Service uninstallation will be handled by external tools (sc.exe)
	return fmt.Errorf("use uninstall-service.bat script to remove the service")
}

func StartService() error {
	return svc.Control(serviceName, svc.Start)
}

func StopService() error {
	return svc.Control(serviceName, svc.Stop)
}
