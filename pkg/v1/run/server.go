package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	s3 "github.com/fehmicorp/s3object/v1/pkg"
	"github.com/getlantern/systray"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

//go:embed app.ico
var iconData []byte

var (
	modkernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procGetConsoleWindow = modkernel32.NewProc("GetConsoleWindow")
	moduser32            = windows.NewLazySystemDLL("user32.dll")
	procShowWindow       = moduser32.NewProc("ShowWindow")
)

const swHide = 0

// HideConsoleWindow hides the black command prompt window for background/silent running
func HideConsoleWindow() {
	if runtime.GOOS == "windows" {
		hwnd, _, _ := procGetConsoleWindow.Call()
		if hwnd != 0 {
			procShowWindow.Call(hwnd, uintptr(swHide))
		}
	}
}

// IsRunningAsAdmin checks if the process is elevated as Administrator
func IsRunningAsAdmin() bool {
	if runtime.GOOS != "windows" {
		return true
	}
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)
	member, err := token.IsMember(sid)
	return err == nil && member
}

// ElevateIfNeeded re-launches the executable with Administrator rights if not already elevated
func ElevateIfNeeded() {
	if runtime.GOOS != "windows" || IsRunningAsAdmin() {
		return
	}

	verb := "runas"
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cwd, _ := os.Getwd()
	args := ""

	verbPtr, _ := windows.UTF16PtrFromString(verb)
	exePtr, _ := windows.UTF16PtrFromString(exe)
	cwdPtr, _ := windows.UTF16PtrFromString(cwd)
	argPtr, _ := windows.UTF16PtrFromString(args)

	var swCommand int32 = 1 // SW_NORMAL

	err = windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, swCommand)
	if err == nil {
		os.Exit(0)
	}
}

// AddToStartup adds the executable to the Windows Registry Run key for automatic startup
func AddToStartup() {
	if runtime.GOOS != "windows" {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	_ = k.SetStringValue("FehmiS3Server", fmt.Sprintf(`"%s"`, exe))
}

func main() {
	// 1. Ensure Windows runs with Administrator privileges
	ElevateIfNeeded()

	// 2. Hide the command prompt window for silent background operation
	HideConsoleWindow()

	// 3. Register application into Windows Startup
	AddToStartup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := 9000
	if p := os.Getenv("S3_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	slug := os.Getenv("S3_SLUG")
	if slug == "" {
		slug = "/dist"
	}

	storagePath := os.Getenv("S3_STORAGE_PATH")
	if storagePath == "" {
		if runtime.GOOS != "windows" {
			storagePath = getStoragePath()
		} else {
			storagePath = "./data"
		}
	}

	engine, err := s3.New(ctx, s3.Options{
		Port:        port,
		InitialSlug: slug,
		StoragePath: storagePath,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Start S3 Engine in background
	go func() {
		if err := engine.Start(ctx); err != nil {
			log.Printf("Server engine stopped: %v", err)
		}
	}()

	// Automatically open index.html in the default browser on Windows
	if runtime.GOOS == "windows" {
		go func() {
			exePath, err := os.Executable()
			var target string
			if err == nil {
				target = filepath.Join(filepath.Dir(exePath), "index.html")
			} else {
				target = "index.html"
			}
			if _, err := os.Stat(target); err == nil {
				cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
				_ = cmd.Start()
			}
		}()
	}

	// Run System Tray UI
	systray.Run(func() {
		systray.SetIcon(iconData)
		systray.SetTitle("Fehmi S3 Server")
		systray.SetTooltip("Fehmi S3 Server (Running)")

		mDocs := systray.AddMenuItem("Documentation", "Open index.html guide")
		mQuit := systray.AddMenuItem("Quit", "Shutdown server and exit")

		go func() {
			for {
				select {
				case <-mDocs.ClickedCh:
					exePath, _ := os.Executable()
					target := filepath.Join(filepath.Dir(exePath), "index.html")
					if _, err := os.Stat(target); os.IsNotExist(err) {
						target = "index.html"
					}
					cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
					_ = cmd.Start()
				case <-mQuit.ClickedCh:
					cancel()
					systray.Quit()
					os.Exit(0)
				}
			}
		}()
	}, func() {
		// Clean up on exit
	})
}

func getStoragePath() string {
	// 1. Scan for any available non-C drive (D through Z)
	for _, letter := range "DEFGHIJKLMNOPQRSTUVWXYZ" {
		driveRoot := fmt.Sprintf("%c:\\", letter)
		if _, err := os.Stat(driveRoot); err == nil {
			// Drive exists and is accessible, create \data folder on it
			targetPath := fmt.Sprintf("%c:\\data", letter)
			if err := os.MkdirAll(targetPath, 0755); err == nil {
				return targetPath
			}
		}
	}
	fallback := `C:\data`
	_ = os.MkdirAll(fallback, 0755)
	return fallback
}
