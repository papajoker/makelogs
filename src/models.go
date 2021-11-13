package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"reflect"
	"regexp"
	"strings"

	"github.com/acarl005/stripansi"
)

var (
	LANG string = getUserLang()
)

type llang struct {
	En string `yaml:"en"`
	De string `yaml:"de"`
	Fr string `yaml:"fr"`
	It string `yaml:"it"`
	Pt string `yaml:"pt"`
	Sp string `yaml:"sp"`
}

// yaml Type gen by: https://zhwt.github.io/yaml-to-go/

type Action struct {
	Name     string `yaml:"name"`
	Command  string `yaml:"command"`
	Object   string `yaml:"object"`
	Type     string `yaml:"type"`
	Level    int    `yaml:"level"`
	Count    int    `yaml:"count"`
	Regex    string `yaml:"regex"`
	Titles   llang  `yaml:"title"`
	Ask      llang  `yaml:"ask"`
	askreply string
	Requires []string `yaml:"require"`
	Pkgs     string   `yaml:"pkgs"`
	Test     string   `yaml:"test"`
	Output   string
	Id       int
}

type Service struct {
	Caption  string
	Version  string
	Command  string
	wantSudo int      `yaml:"sudo"`
	Actions  []Action `yaml:"actions"`
}

func (s *Service) ForEach(function func(action *Action)) {
	for _, action := range s.Actions {
		function(&action)
	}
}

func (s *Service) UseSudo() bool {
	if s.wantSudo == 1 {
		return true
	}
	ok := false
	s.ForEach(func(action *Action) {
		if strings.Contains(action.Command, "sudo") {
			ok = true
			return
		}
	})
	return ok
}

func (a Action) String() string {

	if verboseFlag {
		ty := fmt.Sprintf("\t%-12s\t%s\n", "Type:", a.Type)
		if a.Type == "" {
			ty = ""
		}

		title := a.Titles.GetText()
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
		return fmt.Sprintf("\n::%v%s%v \t%s\n", COLOR_GREEN, a.Name, COLOR_NONE, a.Titles.GetText())
	}
}

// return title, "en" by default
func (l llang) GetText() string {
	ret := l.En

	langs := make(map[string]string)
	v := reflect.ValueOf(l)
	for i := 0; i < v.NumField(); i++ {
		langs[strings.ToUpper(v.Type().Field(i).Name)] = v.Field(i).Interface().(string)
	}

	val, found := langs[LANG]
	if found && val != "" {
		return val
	}
	return ret
}

// dependences are ok for run this action ?
func (a *Action) valid() error {
	for _, req := range a.Requires {
		if strings.HasPrefix(req, "bash:") {
			req = req[5:]
			if a.askreply != "" {
				req = strings.ReplaceAll(req, "%ASK%", a.askreply)
			}
			if exec.Command("bash", "-c", req).Run() != nil {
				return fmt.Errorf("bash condition false \"%s\"", req)
			}
		} else if req[0] == '/' {
			if _, err := os.Stat(req); errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("file not found \"%s\"", req)
			}
		} else {
			req = strings.ToLower(req)
			if a.askreply != "" {
				req = strings.ReplaceAll(req, "%ASK%", a.askreply)
			}
			if exec.Command("bash", "-c", fmt.Sprintf("LANG=C pacman -Qi %s", req)).Run() != nil {
				return fmt.Errorf("package not found \"%s\"", req)
			}
		}
	}
	return nil
}

// run command
func (a *Action) exec() bool {
	a.Output = ""
	defer a.filter()

	// exit if Required not ok
	err := a.valid()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sWarning%s: %s\n", COLOR_BOLD, COLOR_NONE, err)
		return false
	}

	vari := ""
	//get value to include in command
	if a.Test != "" {
		s, _ := exec.Command("bash", "-c", "LANG=C "+a.Test).Output()
		vari = strings.TrimSpace(string(s))
	}

	// get value, prompt a question to user
	if a.askreply != "" {
		vari = a.askreply
	}

	// shell command
	if a.Command != "" {
		cmd := a.Command
		if vari != "" {
			cmd = strings.ReplaceAll(cmd, "%ASK%", vari)
		}
		out, err := exec.Command("bash", "-c", "LANG=C "+cmd+"|cat").Output()
		if err != nil {
			return false
		}
		a.Output = stripansi.Strip(string(out))
		return true
	}

	// or, use object in source code
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
			fmt.Fprint(os.Stderr, err.Error())
			return false
		}
	}
	return false
}

func (a *Action) filter() {
	if a.Output == "" {
		return
	}

	ipv6_regex := `[0-9A-Fa-f]{1,4}:[0-9A-Fa-f]{1,4}:[0-9A-Fa-f]{1,4}:`
	ipv4_regex := `\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`
	mac_regex := `[a-fA-F0-9:]{17}|[a-fA-F0-9]{12}`

	re := regexp.MustCompile(ipv4_regex)
	//a.Output = re.ReplaceAllString(a.Output, "[**ipv4**]")
	submatchall := re.FindAllString(a.Output, -1)
	for _, element := range submatchall {
		fmt.Println(element)
		if strings.HasPrefix(element, "192.168") ||
			strings.HasPrefix(element, "255") ||
			strings.HasPrefix(element, "0.") ||
			strings.HasPrefix(element, "10.") {
			continue
		}
		a.Output = strings.ReplaceAll(a.Output, element, "[**ipv4**]")
	}

	re = regexp.MustCompile(mac_regex)
	a.Output = re.ReplaceAllString(a.Output, "[**filter**]") // mac and ipv6

	re = regexp.MustCompile(ipv6_regex)
	// can exclude fc00... and fe80...
	a.Output = re.ReplaceAllString(a.Output, "[**ipv6**]")

	me, err := user.Current()
	if err == nil {
		a.Output = strings.ReplaceAll(a.Output, me.Username, "[**$USER**]")
	}
}

func getUserLang() string {
	lg := os.Getenv("LANG")
	if len(lg) > 4 {
		return strings.ToUpper(lg[3:5])
	} else {
		return "EN"
	}
}
