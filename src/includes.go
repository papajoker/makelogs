package main

/*
	object to call in yaml files
*/
import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ###############
// Display packages name and version if installed
// ###############

type PkgVer struct {
	pkgs string
}

func (p PkgVer) exec() string {
	cmd := fmt.Sprintf("LANG=C pacman -Qi %s |awk -F':' '/^Name/ {{n=$2}} /^Ver/ {{print n\": \"$2}}'", p.pkgs)
	if p.pkgs != "" {
		out, err := exec.Command("bash", "-c", cmd).Output()
		if err == nil {
			return string(out)
		}
	}
	return ""
}

// ###############
// Display journald log but with error level
// ###############

type JournalType struct {
	UID               string `json:"_UID"`
	Cmdline           string `json:"_CMDLINE,omitempty"`
	SyslogIdentifier  string `json:"SYSLOG_IDENTIFIER"`
	Comm              string `json:"_COMM"`
	RealtimeTimestamp string `json:"__REALTIME_TIMESTAMP"`
	Priority          string `json:"PRIORITY"`
	Message           string `json:"MESSAGE"`
}

type Journald struct {
	level int
	count int
}

func (j Journald) exec() string {
	const f = "__REALTIME_TIMESTAMP,PRIORITY,_COMM,_UID,MESSAGE,_CMDLINE,SYSLOG_IDENTIFIER"
	cmd := fmt.Sprintf("journalctl -b0 -p%d -qr -n%d --no-pager --output-fields=\"%s\" -o json", j.level, j.count, f)
	ret := ""
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err == nil {
		var dat []JournalType
		if err := json.Unmarshal([]byte("["+strings.ReplaceAll(string(out), "}\n{", "},\n{")+"]"), &dat); err != nil {
			panic(err)
		}
		oldentry := ""
		for _, j := range dat {

			i, err := strconv.ParseInt(j.RealtimeTimestamp[0:10], 10, 64)
			if err != nil {
				i = 0
			}
			tm := time.Unix(i, 0)

			/*
				cmdline := j.Cmdline
				if cmdline == "" {
					cmdline = j.SyslogIdentifier
				}
			*/
			entry := fmt.Sprintf("(%s) %s[%s]: %s", j.Priority, j.Comm, j.UID, j.Message)
			// no repeat if same entry
			if entry != oldentry {
				oldentry = entry
				ret = fmt.Sprintf("%s%s %s\n", ret, tm.Format("2006-01-02 15:04:05"), entry)
			}
		}
		return ret
	}
	return ""
}
