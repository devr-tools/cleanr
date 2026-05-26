package setup

import (
	"fmt"
	"os/exec"
	"runtime"
)

var openBrowserURL = defaultOpenBrowserURL

func defaultOpenBrowserURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("browser launch is not supported on %s", runtime.GOOS)
	}
	return cmd.Start()
}
