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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/acarl005/stripansi"
)

const (
	LOGFILE   = "logs.md"
	EXTENSION = "yaml"
)

var (
	Primary   = green // color green
	Secondary = blue  // color blue
	Warning   = red   // color red
	Danger    = red   // color red
	Info      = gray  // color gray
	Hilite    = bold  // bold
)

var (
	red   = Color("\033[1;31m")
	blue  = Color("\033[1;34m")
	green = Color("\033[1;36m")
	bold  = Color("\033[1;1m")
	gray  = Color("\033[38;5;243m")
	//black   = Color("\033[1;30m")
	//yellow  = Color("\033[1;33m")
	//magenta = Color("\033[1;35m")
	//white   = Color("\033[1;37m")

)

func Color(colorCode string) func(...interface{}) string {
	sprint := func(args ...interface{}) string {
		if len(args) == 1 {
			return fmt.Sprintf(colorCode+"%s\033[0m", args[0])
		} else {
			parts := make([]string, len(args))
			for i := 0; i < len(args); i++ {
				parts = append(parts, fmt.Sprintf("%v", args[i]))
			}
			return fmt.Sprintf(colorCode+"%s\033[0m", strings.Join(parts, " "))
		}
	}
	return sprint
}

var (
	GitBranch string
	Version   string
	BuildDate string
	GitID     string
)
var verboseFlag bool = false

func run(conf *Service) {
	//fmt.Printf("%v", conf)
	fmt.Println("--------")
	fmt.Printf("%s \t %s \n\n", Secondary(conf.Caption), conf.Version)

	// Ask before
	for id := range conf.Actions {
		ask := conf.Actions[id].Ask.GetText()
		if ask != "" {
			fmt.Printf("\n%s\n", Primary("##", conf.Actions[id].Name))
			fmt.Printf("%s %s ", Primary("##"), Hilite(ask))
			ret := ""
			fmt.Scanln(&ret)
			if len(ret) > 0 && ret[0] != '.' {
				conf.Actions[id].askreply = strings.TrimSpace(ret)
			}
		}
	}

	var wg sync.WaitGroup

	for id := range conf.Actions {
		wg.Add(1)
		go func(id int, wg *sync.WaitGroup) { // can add go for goroutine ?
			defer wg.Done()
			conf.Actions[id].exec()
		}(id, &wg)
	}
	wg.Wait()
}

func displayShort(conf *Service) {
	fmt.Println(" ")
	fmt.Printf("%s \t%s \t%s", Primary(conf.Command), Info(conf.Caption), Info(conf.Version))

	for _, action := range conf.Actions {
		fmt.Printf("\n\t%-35s %s ", action.Name, Info(action.Titles.GetText()))
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
			fmt.Printf("%s\n%s\n", action, action.Output)
			fmt.Fprintf(f, "\n:: %s\n```\n%v```\n", action.Name, stripansi.Strip(action.Output))
		} else {
			if verbose {
				fmt.Fprintf(os.Stderr, "%s: Nothing for %s\n", Warning("Warning"), action.Name)
			}
		}
	}
	fmt.Printf("\nOutput file : %s\n", Primary(LOGFILE))
}

func searchCommand(search string, configdir *Directory) {
	fmt.Printf("Search: \"%s\"\n", Secondary(search))
	verboseFlag = true
	results := Service{Caption: search}
	i := 0
	r := strings.ReplaceAll(search, " ", "|")
	r = strings.ReplaceAll(r, "+", ".*")
	var validID = regexp.MustCompile(r)
	configdir.ForEachAll(func(conf *Service, action *Action) {
		strf := strings.ToLower(action.Name + " " + action.Titles.GetText() + " " + action.Command)
		if validID.MatchString(strf) {
			i++
			action.Id = i
			results.Actions = append(results.Actions, *action)
		}
	})
	for i, action := range results.Actions {
		t := action.Titles.GetText()
		if t != "" {
			t = "\n   " + t
		}
		fmt.Printf("\n%-3d:: %s%s\n   %s\n", i+1, Primary(action.Name), t, action.Command)
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
		fmt.Fprintf(os.Stderr, "%s: file not found \"%s\"\n", Danger("Error"), logfile)
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
				fmt.Printf("\n:: cloud Url is : %s\n", Primary(string(o)))
				return string(o), nil
			}
		}

		out, err = cloud("ix.io", "'f:1=<-' http://ix.io")
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
				fmt.Fprintf(f, "\n-----\n\n %s\n", out)
			}
		}

		os.Exit(0)
	}
}

func main() {
	var configDir Directory = Directory{}
	configDir.Init(false)

	args := os.Args[1:]
	filename := configDir.Dir + "default." + EXTENSION
	if len(args) > 0 {
		if args[0][0] == '-' {
			// command or option
			helpCmd := flag.Bool("h", false, "Usage")
			listCmd := flag.Bool("l", false, "List logs choice")
			sendCmd := flag.Bool("s", false, "Send log to cloud")
			findCmd := flag.Bool("f", false, "Find/run command")
			lrlistCmd := flag.Bool("lr", false, "List all command for Run")
			rlistCmd := flag.Bool("r", false, "Run commands")
			extractCmd := flag.Bool("e", false, "Extract yaml files")
			flag.BoolVar(&verboseFlag, "v", false, "verbose")
			flag.Parse()

			if *extractCmd {
				configDir.Init(true)
				os.Exit(0)
			}

			if *helpCmd {

				cmd := filepath.Base(os.Args[0])
				fmt.Printf("\n%s Version: %v-%v %v %v\n\n", Primary(cmd), Version, GitID, GitBranch, BuildDate)

				fmt.Println("run -l for list all config available as:")
				fmt.Printf("   ./%s    # load default\n", cmd)
				fmt.Printf("   ./%s wifi\n", cmd)
				fmt.Printf("   ./%s disk\n", cmd)
				fmt.Println("\nREAD/Edit result file:", Hilite(LOGFILE))
				fmt.Printf("\nSend this file to cloud : \"./%s -s\"\n", Hilite(cmd))
				os.Exit(0)
			}

			if *listCmd {
				configDir.ForEach(func(conf *Service) {
					displayShort(conf)
				}, "search")
				os.Exit(0)
			}

			if *lrlistCmd {
				configDir.ForEach(func(conf *Service) {
					s := conf.Command
					for _, action := range conf.Actions {
						c := strings.ReplaceAll(action.Name, " ", "_")
						ret := fmt.Sprintf("%s:%s", Primary(s), c)
						if strings.Index(c, "(") > 0 || strings.Index(c, "?") > 0 {
							ret = fmt.Sprintf("'%s'", ret)
						}
						fmt.Printf("\n %s", ret)
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
				configDir.ForEach(func(conf *Service) {
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
				searchCommand(search, &configDir)
				os.Exit(0)
			}

			if *sendCmd {
				sendToClound(LOGFILE)
				os.Exit(0)
			}

			args = flag.Args()
		}

		// format yaml path/filename
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
				filename = configDir.Dir + "/" + filename
			}
		}
	}

	// run yaml file

	conf := configDir.LoadConf(filename)
	if (conf.UseSudo()) && os.Getuid() != 0 {
		fmt.Fprintf(os.Stderr, "%s: Please start this script as root or sudo!\n", Danger("Error"))
		os.Exit(2)
	}

	start := time.Now()

	run(conf)
	display(conf, true)

	log.Printf("Duration: %s", time.Since(start))

}
