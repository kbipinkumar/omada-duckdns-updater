//go:build windows

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	windowsServiceName = "OmadaDuckDNSUpdater"
	windowsServiceDisp = "Omada DuckDNS Updater"
	windowsServiceDesc = "Fetches WAN IPs from Omada SDN Controller and updates DuckDNS."
)

// windowsService implements the Windows service control handler.
type windowsService struct{}

// Execute runs the updater when started by the Windows Service Control Manager.
func (m *windowsService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- runApp(ctx)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for {
		select {
		case err := <-errCh:
			if err != nil {
				log.Printf("Application error: %v", err)
				errno = 1
			}
			break loop
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				log.Println("Service stop requested")
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				select {
				case <-errCh:
				case <-time.After(10 * time.Second):
				}
				break loop
			default:
				log.Printf("Unexpected service control request: %d", c.Cmd)
			}
		}
	}

	changes <- svc.Status{State: svc.Stopped}
	return false, errno
}

// setupServiceLogging directs log output to updater.log and the Windows event log.
func setupServiceLogging() (io.Closer, error) {
	var closers []io.Closer
	var extra []io.Writer

	elog, err := eventlog.Open(windowsServiceName)
	if err == nil {
		extra = append(extra, &eventLogWriter{elog: elog})
		closers = append(closers, elog)
	}

	fileCloser := initLogging(extra...)
	if fileCloser != nil {
		closers = append(closers, fileCloser)
	}

	return multiCloser(closers), nil
}

// eventLogWriter forwards log lines to the Windows event log.
type eventLogWriter struct {
	elog *eventlog.Log
}

// Write records p as an informational Windows event log entry.
func (w *eventLogWriter) Write(p []byte) (int, error) {
	msg := string(p)
	if err := w.elog.Info(1, msg); err != nil {
		return 0, err
	}
	return len(p), nil
}

// multiCloser closes a slice of io.Closer values, returning the first error.
type multiCloser []io.Closer

// Close closes every closer in m and returns the first error encountered.
func (m multiCloser) Close() error {
	var first error
	for _, c := range m {
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// runMaybeAsService runs in the foreground or as a Windows service when applicable.
func runMaybeAsService() error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return fmt.Errorf("detect windows service: %w", err)
	}
	if !isService {
		return runConsole()
	}

	closer, _ := setupServiceLogging()
	if closer != nil {
		defer closer.Close()
	}

	return svc.Run(windowsServiceName, &windowsService{})
}

// handleServiceCommand processes Windows service install and control commands.
func handleServiceCommand(cmd string) error {
	switch cmd {
	case "install":
		return installService()
	case "uninstall":
		return uninstallService()
	case "start":
		return startService()
	case "stop":
		return stopService()
	default:
		return fmt.Errorf("unknown service command %q (use install, uninstall, start, or stop)", cmd)
	}
}

// exePath returns the absolute path to the running executable.
func exePath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(p)
}

// installService registers the updater as a Windows service.
func installService() error {
	exepath, err := exePath()
	if err != nil {
		return err
	}

	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(windowsServiceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", windowsServiceName)
	}

	s, err = m.CreateService(windowsServiceName, exepath, mgr.Config{
		DisplayName: windowsServiceDisp,
		Description: windowsServiceDesc,
		StartType:   mgr.StartAutomatic,
		Dependencies: []string{"Tcpip"},
	})
	if err != nil {
		return err
	}
	defer s.Close()

	err = eventlog.InstallAsEventCreate(windowsServiceName, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return fmt.Errorf("SetupEventLogSource failed: %w", err)
	}

	fmt.Printf("Service %s installed successfully.\n", windowsServiceName)
	return nil
}

// uninstallService removes the updater Windows service registration.
func uninstallService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(windowsServiceName)
	if err != nil {
		return fmt.Errorf("service %s is not installed", windowsServiceName)
	}
	defer s.Close()

	_ = stopServiceHandle(s)

	if err = s.Delete(); err != nil {
		return err
	}

	_ = eventlog.Remove(windowsServiceName)
	fmt.Printf("Service %s uninstalled successfully.\n", windowsServiceName)
	return nil
}

// startService starts the installed Windows service.
func startService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(windowsServiceName)
	if err != nil {
		return fmt.Errorf("could not open service: %w", err)
	}
	defer s.Close()

	if err = s.Start(); err != nil {
		return fmt.Errorf("could not start service: %w", err)
	}
	fmt.Printf("Service %s started.\n", windowsServiceName)
	return nil
}

// stopService stops the installed Windows service.
func stopService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(windowsServiceName)
	if err != nil {
		return fmt.Errorf("could not open service: %w", err)
	}
	defer s.Close()

	if err = stopServiceHandle(s); err != nil {
		return err
	}
	fmt.Printf("Service %s stopped.\n", windowsServiceName)
	return nil
}

// stopServiceHandle sends a stop control request and waits for the service to stop.
func stopServiceHandle(s *mgr.Service) error {
	status, err := s.Control(svc.Stop)
	if err != nil {
		return err
	}
	timeout := time.Now().Add(10 * time.Second)
	for status.State != svc.Stopped {
		if time.Now().After(timeout) {
			return fmt.Errorf("timeout waiting for service to stop")
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return err
		}
	}
	return nil
}
