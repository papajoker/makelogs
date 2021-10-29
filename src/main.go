package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/acarl005/stripansi"
	"gopkg.in/yaml.v3"
)

//go:embed yaml/*.yaml
var fe embed.FS

const (
	//https://misc.flogisoft.com/bash/tip_colors_and_formatting
	COLOR_NONE  = "\033[0m"
	COLOR_BLUE  = "\033[0;34m"
	COLOR_GREEN = "\033[0;36m"
	COLOR_RED   = "\033[38;5;124m"
	COLOR_GRAY  = "\033[38;5;243m"
	_VERSION    = "0.0.4"
	LOGFILE     = "logs.md"
	EXTENSION   = "yaml"
)

var verboseFlag bool = false

//var includeCommands = [2]string{"PkgVer", "Journald"}

// yaml Type gen by: https://zhwt.github.io/yaml-to-go/
type Action struct {
	Name    string `yaml:"name"`
	Command string `yaml:"command"`
	Object  string `yaml:"object"`
	Type    string `yaml:"type"`
	Level   int    `yaml:"level"`
	Count   int    `yaml:"count"`
	Titles  struct {
		En string `yaml:"en"`
		De string `yaml:"de"`
		Fr string `yaml:"fr"`
		It string `yaml:"it"`
		Pt string `yaml:"pt"`
		Sp string `yaml:"sp"`
	} `yaml:"title"`
	Requires []string `yaml:"require"`
	Pkgs     string   `yaml:"pkgs"`
	Output   string
}

type Service struct {
	Caption  string
	Version  string
	WantSudo int      `yaml:"sudo"`
	Actions  []Action `yaml:"actions"`
}

func (a Action) String() string {

	if verboseFlag {
		ty := fmt.Sprintf("\t%-12s\t%s\n", "Type:", a.Type)

		title := a.getTitle()
		if title != "" {
			title = fmt.Sprintf("\t%-12s\t%s\n", "Title:", title)
		} else {
			title = ""
		}

		req := ""
		if len(a.Requires) > 0 {
			req = fmt.Sprintf("\t%-12s\t%v\n", "Require:", a.Requires)
		}

		pkgs := ""
		if a.Pkgs != "" {
			pkgs = fmt.Sprintf("\t%-12s\t%v\n", "Packages:", a.Pkgs)
		}

		le := ""
		c := ""
		if a.Object == "Journald" {
			le = fmt.Sprintf("\t%-12s\t%v\n", "Level logs:", a.Level)
			c = fmt.Sprintf("\t%-12s\t%v\n", "max:", a.Count)
		}

		ob := ""
		if a.Object != "" {
			ob = fmt.Sprintf("\t%-12s\t%v\n", "Object:", a.Object)
		} else {
			ob = fmt.Sprintf("\t%-12s\t%v\n", "Command:", a.Command)
		}
		return fmt.Sprintf("\n::%v%s%v \n%s %s %s %s %s %s %s", COLOR_GREEN, a.Name, COLOR_NONE, title, ty, ob, req, pkgs, le, c)
	} else {
		return fmt.Sprintf("\n::%v%s%v \t%s\n", COLOR_GREEN, a.Name, COLOR_NONE, a.getTitle())
	}
}

func (a Action) getTitle() string {
	ret := a.Titles.En

	langs := make(map[string]string)
	v := reflect.ValueOf(a.Titles)
	for i := 0; i < v.NumField(); i++ {
		langs[strings.ToUpper(v.Type().Field(i).Name)] = v.Field(i).Interface().(string)
	}

	lg := os.Getenv("LANG")
	if len(lg) > 4 {
		lg = strings.ToUpper(os.Getenv("LANG")[3:5])
	} else {
		lg = "EN"
	}

	val, found := langs[lg]
	if found && val != "" {
		return val
	}
	return ret
}

// run command
func (a *Action) exec() bool {
	a.Output = ""

	// packages and files installed ?
	for _, req := range a.Requires {
		if req[0] == '/' {
			if _, err := os.Stat(req); errors.Is(err, fs.ErrNotExist) {
				fmt.Fprintf(os.Stderr, "%sWarning%s: file not found \"%s\"\n", COLOR_RED, COLOR_NONE, req)
				return false
			}
		} else {
			req = strings.ToLower(req)
			_, err := exec.Command("bash", "-c", fmt.Sprintf("LANG=C pacman -Qi %s", req)).Output()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%sWarning%s: package not found \"%s\"\n", COLOR_RED, COLOR_NONE, req)
				return false
			}
		}
	}

	// shell command
	if a.Command != "" {
		out, err := exec.Command("bash", "-c", "LANG=C "+a.Command+"|cat").Output()
		if err != nil {
			return false
			//panic(fmt.Sprintf("Error in %s", a.Name))
		}
		a.Output = stripansi.Strip(string(out))
		return true
	} else {
		// use object in source code
		// TODO use includeCommands ?s
		if a.Object != "" {
			switch a.Object {
			case "PkgVer":
				obj := PkgVer{pkgs: strings.ToLower(a.Pkgs)}
				out := obj.exec()
				if out != "" {
					a.Output = stripansi.Strip(out)
					return true
				}
			case "Journald":
				if a.Level == 0 {
					a.Level = 3
				}
				if a.Count == 0 {
					a.Count = 32
				}
				obj := Journald{level: a.Level, count: a.Count}
				out := obj.exec()
				if out != "" {
					a.Output = stripansi.Strip(out)
					return true
				}
			default:
				fmt.Fprintf(os.Stderr, "Warning: function \"%s\" not in App\n", a.Object)
				return false
			}
		}
	}
	return false
}

// -------------- INCLUDE OBJECT --------------------- //

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

// Display journald log but with error level

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

// -------------- MAIN --------------------- //

func run(conf *Service) {
	//fmt.Printf("%v", conf)
	fmt.Println("--------")
	fmt.Printf("%v%s%v \t %s \n\n", COLOR_BLUE, conf.Caption, COLOR_NONE, conf.Version)

	var wg sync.WaitGroup

	for id := range conf.Actions {
		wg.Add(1)
		// 440 ms sans goroutine
		// 320 ms avec goroutine
		go func(id int, wg *sync.WaitGroup) { // can add go for goroutine ?
			defer wg.Done()
			action := conf.Actions[id]
			action.exec()
			conf.Actions[id] = action
		}(id, &wg)
	}
	wg.Wait()
}

func displayShort(filename string, conf *Service) {
	//fmt.Printf("%v", conf)
	fmt.Println(" ")
	fmt.Printf("%v%s%v \t%v%s%v \t%s%s%s", COLOR_GREEN, filename[:len(filename)-5], COLOR_NONE, COLOR_GRAY, conf.Caption, COLOR_NONE, COLOR_GRAY, conf.Version, COLOR_NONE)

	for _, action := range conf.Actions {
		fmt.Printf("\n\t%-35s %s%s%s ", action.Name, COLOR_GRAY, action.getTitle(), COLOR_NONE)
		//fmt.Println(action)
	}
	fmt.Println("")
}

func display(conf *Service) {
	f, err := os.Create(LOGFILE)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	fmt.Fprintf(f, "### %s\n", conf.Caption)

	for _, action := range conf.Actions {
		if action.Output != "" {
			fmt.Printf("%s\n%v\n", action, action.Output)
			fmt.Fprintf(f, "\n:: %s\n```\n%v```\n", action.Name, action.Output)
		} else {
			fmt.Fprintf(os.Stderr, "%sWarning%s: Nothing for %s\n", COLOR_RED, COLOR_NONE, action.Name)
		}
	}
}

func loadConf(filename string) *Service {
	yfile, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}

	conf := &Service{}
	err1 := yaml.Unmarshal(yfile, conf)
	if err1 != nil {
		log.Fatal(err1)
	}
	return conf
}

func sendToClound(logfile string) {
	if _, err := os.Stat(logfile); errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "%sError%s: file not found \"%s\"\n", COLOR_RED, COLOR_NONE, logfile)
		os.Exit(1)
	}
	fmt.Printf("! Read log \"%s\" before send this file on web\n", logfile)
	fmt.Println("Send ? (y/N)")
	var input string
	_, err := fmt.Scanln(&input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(127)
	}
	if strings.ToUpper(input) == "Y" {

		cloud := func(name string, url string) (err error) {
			cmd := fmt.Sprintf("cat '%s' | curl -s -F %s", logfile, url)
			out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
			if err != nil {
				return fmt.Errorf("error %s : %s - %s", name, string(out), err)
			} else {
				fmt.Printf("\n:: cloud url is : %v%s%v\f", COLOR_GREEN, string(out), COLOR_NONE)
				return nil
			}
		}

		if err := cloud("ix.io", "'f:1;read:1=<-' http://ix.io"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			//TODO rm read:1
			if err := cloud("sprunge", "'sprunge=<-' http://sprunge.us?md"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}

		os.Exit(0)
	}
}

func extractConf(configdir string, dir embed.FS) {
	myDirFiles, _ := dir.ReadDir(EXTENSION)
	for _, de := range myDirFiles {
		f, err := os.Create(configdir + de.Name())
		if err != nil {
			log.Fatal(err)
		}
		fdata, err := dir.ReadFile(EXTENSION + "/" + de.Name())
		if err != nil {
			log.Fatal(err)
		}
		fmt.Fprint(f, string(fdata))
	}
}

func main() {
	fmt.Println("makelogs", _VERSION)

	configdir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	configdir = configdir + "/.local/share/makelogs/"

	/*
		TODO
			if not found /var/$configdir => script not installed by pacman:
					extract yaml resources
			else
				use files in /var/
	*/

	if _, err := os.Stat(configdir); errors.Is(err, fs.ErrNotExist) {
		// extract resources only at first run ... ?
		// TODO unsecure to have these scripts in home ... extract allways / use only embed files ? or use /var/
		os.MkdirAll(configdir, os.ModePerm)
		extractConf(configdir, fe)
	}

	args := os.Args[1:]
	filename := configdir + "default." + EXTENSION
	if len(args) > 0 {
		if args[0][0] == '-' {
			// command or option
			helpCmd := flag.Bool("h", false, "Usage")
			listCmd := flag.Bool("l", false, "List logs choice")
			sendCmd := flag.Bool("s", false, "Send log to cloud")
			flag.BoolVar(&verboseFlag, "v", false, "verbose")

			flag.Parse()
			//fmt.Println(flag.Args())

			if *helpCmd {
				fmt.Println("run -l for list all config available as:")
				fmt.Println("\t./makelogs # load default")
				fmt.Println("\t./makelogs wifi")
				fmt.Println("\t./makelogs disk")
				fmt.Println("\nREAD/Edit result file:", LOGFILE)
				fmt.Println("\nSend this file to cloud : \"./makelogs -s\"")
				os.Exit(0)
			}
			if *listCmd {
				matches, err := filepath.Glob(configdir + "/*." + EXTENSION)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				for _, filename := range matches {
					conf := loadConf(filename)
					displayShort(path.Base(filename), conf)
				}
				os.Exit(0)
			}
			if *sendCmd {
				sendToClound(LOGFILE)
				os.Exit(0)
			}

			args = flag.Args()
			/*
				if len(args) < 1 {
					os.Exit(0)
				}
			*/
		}
		if len(args) > 0 && args[0][0] != '-' {
			filename = args[0]
			if !strings.HasSuffix(filename, "."+EXTENSION) {
				filename += "." + EXTENSION
			}
			if strings.HasPrefix(filename, ".") {
				pwd, _ := os.Getwd()
				filename = pwd + "/" + filename
			}
			if !strings.HasPrefix(filename, "/") {
				filename = configdir + "/" + filename
			}
		}
	}

	conf := loadConf(filename)

	if conf.WantSudo == 1 && os.Getuid() != 0 {
		fmt.Fprintf(os.Stderr, "%sError%s: Please start this script as root or sudo!\n", COLOR_RED, COLOR_NONE)
		os.Exit(2)
	}

	start := time.Now()

	run(conf)
	display(conf)

	log.Printf("Duration: %s", time.Since(start))

}
