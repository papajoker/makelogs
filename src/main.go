package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/acarl005/stripansi"
)

const (
	//https://misc.flogisoft.com/bash/tip_colors_and_formatting
	COLOR_NONE  = "\033[0m"
	COLOR_BLUE  = "\033[0;34m"
	COLOR_GREEN = "\033[0;36m"
	COLOR_RED   = "\033[38;5;124m"
	COLOR_GRAY  = "\033[38;5;243m"
	_VERSION    = "0.1.0"
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
	Regex   string `yaml:"regex"`
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
	Id       int
}

type Service struct {
	Caption  string
	Version  string
	Command  string
	WantSudo int      `yaml:"sudo"`
	Actions  []Action `yaml:"actions"`
}

func (s *Service) ForEach(function func(action *Action)) {
	for _, action := range s.Actions {
		function(&action)
	}
}

func (a Action) String() string {

	if verboseFlag {
		ty := fmt.Sprintf("\t%-12s\t%s\n", "Type:", a.Type)
		if a.Type == "" {
			ty = ""
		}

		title := a.GetTitle()
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
		return fmt.Sprintf("\n::%v%s%v \t%s\n", COLOR_GREEN, a.Name, COLOR_NONE, a.GetTitle())
	}
}

func (a Action) GetTitle() string {
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
					a.Output = out
					return true
				}
			case "LogsActivity":
				if a.Count == 0 {
					a.Count = 30
				}
				if a.Regex == "" {
					a.Regex = ".*"
				}
				obj := LogsActivity{count: a.Count, regex: a.Regex}
				out := obj.exec()
				if out != "" {
					a.Output = out
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

func displayShort(conf *Service) {
	//fmt.Printf("%v", conf)
	fmt.Println(" ")
	fmt.Printf("%v%s%v \t%v%s%v \t%s%s%s", COLOR_GREEN, conf.Command, COLOR_NONE, COLOR_GRAY, conf.Caption, COLOR_NONE, COLOR_GRAY, conf.Version, COLOR_NONE)

	for _, action := range conf.Actions {
		fmt.Printf("\n\t%-35s %s%s%s ", action.Name, COLOR_GRAY, action.GetTitle(), COLOR_NONE)
		//fmt.Println(action)
	}
	fmt.Println("")
}

func display(conf *Service, verbose bool) {
	f, err := os.Create(LOGFILE)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	fmt.Fprintf(f, "### %s\n", conf.Caption)

	for _, action := range conf.Actions {
		if action.Output != "" {
			fmt.Printf("%s\n%v\n", action, action.Output)
			fmt.Fprintf(f, "\n:: %s\n```\n%v```\n", action.Name, stripansi.Strip(action.Output))
		} else {
			if verbose {
				fmt.Fprintf(os.Stderr, "%sWarning%s: Nothing for %s\n", COLOR_RED, COLOR_NONE, action.Name)
			}
		}
	}
}

func searchCommand(search string, configdir *Directory) {
	fmt.Printf("Search: \"%v%s%v\"\n", COLOR_BLUE, search, COLOR_NONE)
	verboseFlag = true
	results := Service{Caption: search}
	i := 0
	r := strings.ReplaceAll(search, " ", "|")
	r = strings.ReplaceAll(r, "+", ".*")
	var validID = regexp.MustCompile(r)
	configdir.ForEachAll(func(conf *Service, action *Action) {
		strf := strings.ToLower(action.Name + " " + action.GetTitle() + " " + action.Command)
		if validID.MatchString(strf) {
			i++
			action.Id = i
			results.Actions = append(results.Actions, *action)
		}
	})
	for i, action := range results.Actions {
		t := action.GetTitle()
		if t != "" {
			t = "\n   " + t
		}
		fmt.Printf("\n%-3d:: %v%s%v%s\n   %s\n", i+1, COLOR_GREEN, action.Name, COLOR_NONE, t, action.Command)
	}
	fmt.Println("")
	if len(results.Actions) > 0 {
		fmt.Printf("Command to run ? (1..%d) ", len(results.Actions))

		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			fmt.Println("")
			for _, number := range strings.Fields(scanner.Text()) {
				id, err := strconv.Atoi(number)
				if err != nil || id < 1 || id > len(results.Actions) {
					continue
				}
				results.Actions[id-1].exec()
			}

			display(&results, false)
		}
	}
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

func main() {
	configdir := Directory{}
	configdir.Init()
	fmt.Println("makelogs", _VERSION)

	args := os.Args[1:]
	filename := configdir.Dir + "default." + EXTENSION
	if len(args) > 0 {
		if args[0][0] == '-' {
			// command or option
			helpCmd := flag.Bool("h", false, "Usage")
			listCmd := flag.Bool("l", false, "List logs choice")
			sendCmd := flag.Bool("s", false, "Send log to cloud")
			findCmd := flag.Bool("f", false, "find  a command")
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
				configdir.ForEach(func(conf *Service) {
					displayShort(conf)
				}, "search")
				os.Exit(0)
			}
			if *findCmd {
				if len(flag.Args()) < 1 {
					os.Exit(127)
				}
				search := strings.ToLower(strings.Join(flag.Args(), " "))
				if len(search) < 3 {
					os.Exit(127)
				}
				searchCommand(search, &configdir)
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
				filename = configdir.Dir + "/" + filename
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
	display(conf, true)

	log.Printf("Duration: %s", time.Since(start))

}
