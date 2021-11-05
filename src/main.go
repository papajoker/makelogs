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
	"path/filepath"
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
	LOGFILE     = "logs.md"
	EXTENSION   = "yaml"
)

var (
	GitBranch string
	Version   string
	BuildDate string
	GitID     string
)
var verboseFlag bool = false

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

	// exit if Required not ok
	for _, req := range a.Requires {
		if strings.HasPrefix(req, "bash:") {
			req = req[5:]
			if exec.Command("bash", "-c", req).Run() != nil {
				fmt.Fprintf(os.Stderr, "%sWarning%s: bash condition false \"%s\"\n", COLOR_RED, COLOR_NONE, req)
				return false
			}
		} else if req[0] == '/' {
			if _, err := os.Stat(req); errors.Is(err, fs.ErrNotExist) {
				fmt.Fprintf(os.Stderr, "%sWarning%s: file not found \"%s\"\n", COLOR_RED, COLOR_NONE, req)
				return false
			}
		} else {
			req = strings.ToLower(req)
			if exec.Command("bash", "-c", fmt.Sprintf("LANG=C pacman -Qi %s", req)).Run() != nil {
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
		}
		a.Output = stripansi.Strip(string(out))
		return true
	}

	// use object in source code
	if a.Object != "" {
		obj, err := Objectfactory(a.Object)
		if err == nil {
			obj.init(a)
			out := obj.exec()
			if out != "" {
				a.Output = stripansi.Strip(out)
				return true
			}
		} else {
			fmt.Fprintf(os.Stderr, err.Error())
			return false
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
	fmt.Printf("\nOutput file : %v%s%v\n", COLOR_GREEN, LOGFILE, COLOR_NONE)
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

	out := ""
	if strings.ToUpper(input) == "Y" {

		cloud := func(name string, url string) (string, error) {
			cmd := fmt.Sprintf("cat '%s' | curl -s -F %s", logfile, url)
			o, e := exec.Command("bash", "-c", cmd).CombinedOutput()
			if e != nil {
				return "", fmt.Errorf("error %s : %s - %s", name, string(o), e)
			} else {
				fmt.Printf("\n:: cloud url is : %v%s%v\n", COLOR_GREEN, string(o), COLOR_NONE)
				return string(o), nil
			}
		}

		//TODO rm read:2
		out, err = cloud("ix.io", "'f:1;read:1=<-' http://ix.io")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			out, err = cloud("sprunge", "'sprunge=<-' http://sprunge.us?md")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}

		if out != "" {
			f, err := os.OpenFile(logfile, os.O_APPEND|os.O_WRONLY, 0644)
			if err == nil {
				defer f.Close()
				fmt.Fprintf(f, "\n-----\n\nUrl : %s\n", out)
			}
		}

		os.Exit(0)
	}
}

func main() {
	configdir := Directory{}
	configdir.Init()

	args := os.Args[1:]
	filename := configdir.Dir + "default." + EXTENSION
	if len(args) > 0 {
		if args[0][0] == '-' {
			// command or option
			helpCmd := flag.Bool("h", false, "Usage")
			listCmd := flag.Bool("l", false, "List logs choice")
			sendCmd := flag.Bool("s", false, "Send log to cloud")
			findCmd := flag.Bool("f", false, "Find/run command")
			lrlistCmd := flag.Bool("lr", false, "List all command for Run")
			rlistCmd := flag.Bool("r", false, "Run commands")
			flag.BoolVar(&verboseFlag, "v", false, "verbose")
			flag.Parse()

			if *helpCmd {

				cmd := filepath.Base(os.Args[0])
				fmt.Printf("\n%v%s%v Version: %v-%v %v %v\n\n", COLOR_GREEN, cmd, COLOR_NONE, Version, GitID, GitBranch, BuildDate)

				fmt.Println("run -l for list all config available as:")
				fmt.Printf("   ./%s    # load default\n", cmd)
				fmt.Printf("   ./%s wifi\n", cmd)
				fmt.Printf("   ./%s disk\n", cmd)
				fmt.Println("\nREAD/Edit result file:", LOGFILE)
				fmt.Printf("\nSend this file to cloud : \"./%s -s\"\n", cmd)
				os.Exit(0)
			}
			if *listCmd {
				configdir.ForEach(func(conf *Service) {
					displayShort(conf)
				}, "search")
				os.Exit(0)
			}
			if *lrlistCmd {
				configdir.ForEach(func(conf *Service) {
					s := conf.Command
					for _, action := range conf.Actions {
						c := strings.ReplaceAll(action.Name, " ", "_")
						ret := fmt.Sprintf("%v%s%v:%s", COLOR_GREEN, s, COLOR_NONE, c)
						if strings.Index(c, "(") > 0 {
							ret = fmt.Sprintf("'%s'", ret)
						}
						fmt.Printf("\n%s", ret)
					}
				}, "*")
				fmt.Println("")
				os.Exit(0)
			}
			if *rlistCmd {
				// -r  "default:memory_(base_10)" pacman:arch

				results := Service{Caption: "My logs"}
				args = flag.Args()

				fmt.Println(args)
				configdir.ForEach(func(conf *Service) {
					s := conf.Command
					for _, action := range conf.Actions {
						canr := fmt.Sprintf("%s:%s", s, strings.ReplaceAll(action.Name, " ", "_"))
						for _, v := range args {
							if canr == v {
								fmt.Printf("\n %s", canr)
								results.Actions = append(results.Actions, action)
								break
							}
						}

					}
				}, "*")
				fmt.Println("")
				run(&results)
				display(&results, true)
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
