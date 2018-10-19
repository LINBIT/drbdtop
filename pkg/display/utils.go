/*
 *drbdtop - statistics for DRBD
 *Copyright © 2017 Hayley Swimelar and Roland Kammerer
 *
 *This program is free software; you can redistribute it and/or modify
 *it under the terms of the GNU General Public License as published by
 *the Free Software Foundation; either version 2 of the License, or
 *(at your option) any later version.
 *
 *This program is distributed in the hope that it will be useful,
 *but WITHOUT ANY WARRANTY; without even the implied warranty of
 *MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *GNU General Public License for more details.
 *
 *You should have received a copy of the GNU General Public License
 *along with this program; if not, see <http://www.gnu.org/licenses/>.
 */

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

	hostname := "unknown"
	if h, err := os.Hostname(); err == nil {
		hostname = h
	}

	return fmt.Sprintf("(kernel: %s; utils: %s; host: %s)", kernel, utils, hostname)
}

func dmesg(res string) ([]string, error) {
	dmesgCmd := exec.Command("dmesg")
	grepCmd := exec.Command("grep", res)

	var err error
	if grepCmd.Stdin, err = dmesgCmd.StdoutPipe(); err != nil {
		return nil, err
	}

	if err = dmesgCmd.Start(); err != nil {
		return nil, err
	}

	defer dmesgCmd.Wait()

	var buf []byte
	if buf, err = grepCmd.Output(); err != nil {
		return nil, err
	}

	return strings.Split(string(buf), "\n"), nil
}
