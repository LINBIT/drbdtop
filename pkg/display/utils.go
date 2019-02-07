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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type version struct {
	major, minor, patch int
}

func getVersion(field string) (version, error) {
	field += "="

	var ver version

	cmd := exec.Command("drbdadm", "--version")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ver, err
	}
	if err := cmd.Start(); err != nil {
		return ver, err
	}

	scanner := bufio.NewScanner(bufio.NewReader(stdout))

	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		txt := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(txt, field) {
			code64, err := strconv.ParseInt(strings.TrimPrefix(txt, field), 0, 32)
			code := int(code64)
			if err != nil {
				return ver, err
			}
			ver.major = ((code >> 16) & 0xff)
			ver.minor = ((code >> 8) & 0xff)
			ver.patch = (code & 0xff)
			return ver, nil
		}
	}
	return ver, fmt.Errorf("Could not find field '%s'", field)
}

func getUtilsVersion() (version, error) { return getVersion("DRBDADM_VERSION_CODE") }

func getKernelModVersion() (version, error) {
	if kver, err := getVersion("DRBD_KERNEL_VERSION_CODE"); err == nil {
		return kver, err
	}

	// fall back to /proc/drbd
	var kver version
	file, err := os.Open("/proc/drbd")
	if err != nil {
		return kver, err
	}

	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		txt := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(txt, "version:") {
			fields := strings.Split(txt, " ")
			if len(fields) >= 2 {
				// version: 9.0.16-0rc2 (api:...)
				kv := strings.SplitN(fields[1], "-", 2)
				kv = strings.Split(kv[0], ".")
				var err error
				if kver.major, err = strconv.Atoi(kv[0]); err != nil {
					return kver, err
				}
				if kver.minor, err = strconv.Atoi(kv[1]); err != nil {
					return kver, err
				}
				kver.patch, err = strconv.Atoi(kv[2])
				return kver, err
			}
			break // we tried once, give up
		}
	}

	return kver, errors.New("Could not determine kernel version")
}

func getVersionInfo() string {
	utils := "unknown"
	if utv, err := getUtilsVersion(); err == nil {
		utils = fmt.Sprintf("%d.%d.%d", utv.major, utv.minor, utv.patch)
	}

	kernel := "unknown"
	if kv, err := getKernelModVersion(); err == nil {
		kernel = fmt.Sprintf("%d.%d.%d", kv.major, kv.minor, kv.patch)
	}

	hostname := "unknown"
	if h, err := os.Hostname(); err == nil {
		hostname = h
	}

	return fmt.Sprintf("(kernel: %s; utils: %s; host: %s)", kernel, utils, hostname)
}

// IsBlacklistedVersion returns an error if the detected versions are bad (e.g., kernel so old it dies not support events2).
func IsBlacklistedVersion() error {
	if kv, err := getKernelModVersion(); err != nil {
		return err
	} else if kv.major == 8 && ((kv.minor < 4) || (kv.minor == 4 && kv.patch < 6)) { // events2
		return fmt.Errorf("DRBD kernel module (%d.%d.%d) too old, please use at least DRBD 8.4.6",
			kv.major, kv.minor, kv.patch)
	}

	return nil
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
