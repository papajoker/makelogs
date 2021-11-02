package main

/*
	object to call in yaml files
*/
import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ObjectLog interface {
	exec() string
	init(a *Action)
}

// ###############
// Display packages name and version if installed
// ###############

type PkgVer struct {
	pkgs string
}

func (p *PkgVer) init(a *Action) {
	p.pkgs = strings.ToLower(a.Pkgs)
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

func (j *Journald) init(a *Action) {
	if a.Level == 0 {
		a.Level = 3
	}
	if a.Count == 0 {
		a.Count = 32
	}
	j.count = a.Count
	j.level = a.Level
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

type LogsActivity struct {
	count int
	regex *regexp.Regexp
}

/*
[2021-10-28T04:01:19+0200] [PACMAN] starting full system upgrade
[2021-10-28T10:11:07+0200] [ALPM] transaction started
[2021-10-31T01:22:07+0200] [ALPM] upgraded xmlsec (1.2.32-1 -> 1.2.33-1)
[2021-08-06T21:08:06+0200] [ALPM] removed tk (8.6.11.1-1)
[2021-08-05T21:05:23+0200] [ALPM] installed libtg_owt (0.git6.91d836d-2)
[2021-10-30T15:30:22+0200] [ALPM] transaction completed
*/

func (l *LogsActivity) init(a *Action) {
	if a.Count == 0 {
		a.Count = 30
	}
	if a.Regex == "" {
		a.Regex = ".*"
	}
	l.count = a.Count
	l.regex = regexp.MustCompile(a.Regex)
}

func (l LogsActivity) exec() (ret string) {
	now := time.Now().AddDate(0, -0, -l.count)

	file, err := os.Open("/var/log/pacman.log")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	calendar := make(map[string]int)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 20 && line[0] == '[' {
			d := line[1:11]
			t, _ := time.Parse("2006-01-02", d)
			if t.After(now) {
				if strings.Index(line, "[ALPM] ") == -1 {
					continue
				}
				//run regex ...
				if l.regex.MatchString(line) {
					if _, ok := calendar[d]; ok {
						calendar[d] += 1
					} else {
						calendar[d] = 1
					}
				}
			}
		}
	}
	max := func() (max int) {
		for _, v := range calendar {
			if v > max {
				max = v
			}
		}
		return max
	}()
	sortc := func(map[string]int) []string {
		keys := make([]string, len(calendar))
		i := 0
		for k := range calendar {
			keys[i] = k
			i++
		}
		sort.Strings(keys)
		return keys
	}
	for _, i := range sortc(calendar) {
		c := calendar[i]
		pourcent := int(math.Round((float64(c) * float64(100)) / float64(max)))
		ret += fmt.Sprintf("%s %v%-3v%v %s\n", i, COLOR_GREEN, c, COLOR_NONE, strings.Repeat("━", pourcent))
	}
	return ret
}
