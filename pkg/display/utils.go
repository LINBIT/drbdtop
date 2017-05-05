package display

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func getVersionInfo() string {
	utils := "unknown"
	kernel := "unknown"

	DRBD_KERNEL_VERSION := "DRBD_KERNEL_VERSION="
	DRBD_UTILS_VERSION := "DRBDADM_VERSION="

	cmd := exec.Command("drbdadm", "--version")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ""
	}
	if err := cmd.Start(); err != nil {
		return ""
	}

	reader := bufio.NewReader(stdout)
	scanner := bufio.NewScanner(reader)

	scanner.Split(bufio.ScanLines)

	var u, k bool
	for scanner.Scan() {
		txt := strings.TrimSpace(scanner.Text())
		if !k && strings.HasPrefix(txt, DRBD_KERNEL_VERSION) {
			kernel = "v" + strings.TrimPrefix(txt, DRBD_KERNEL_VERSION)
			k = true
		}
		if !u && strings.HasPrefix(txt, DRBD_UTILS_VERSION) {
			utils = "v" + strings.TrimPrefix(txt, DRBD_UTILS_VERSION)
			u = true
		}
		if k && u {
			break
		}
	}

	if !k { // fall back to /proc/drbd
		file, err := os.Open("/proc/drbd")
		if err == nil {
			defer file.Close()
			scanner = bufio.NewScanner(file)
			for scanner.Scan() {
				txt := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(txt, "version:") {
					fields := strings.Split(txt, " ")
					if len(fields) >= 2 {
						kernel = fields[1]
					}
				}
				break
			}

		}
	}

	return fmt.Sprintf("(kernel: %s/utils: %s)", kernel, utils)
}
