package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed yaml/*.yaml
var fe embed.FS

type Directory struct {
	Dir string
}

func (d *Directory) Init(force bool) {
	configdir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	// TODO		use /tmp/makelogs ? or not extract is the best ;)
	d.Dir = configdir + "/.local/share/makelogs/"

	if _, err := os.Stat(d.Dir); errors.Is(err, fs.ErrNotExist) {
		// extract resources only at first run ... ?
		// TODO unsecure to have these scripts in home ... extract allways / use only embed files ? or use /var/
		os.MkdirAll(d.Dir, os.ModePerm)
		extractConf(d.Dir, fe)
	}
	if force {
		err := os.RemoveAll(d.Dir)
		if err != nil {
			fmt.Println(err)
			os.Exit(4)
		}
		os.MkdirAll(d.Dir, os.ModePerm)
		extractConf(d.Dir, fe)
	}
}

func (d Directory) ForEach(function func(conf *Service), exclude string) {
	matches, err := filepath.Glob(d.Dir + "/*." + EXTENSION)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	for _, filename := range matches {
		conf := d.LoadConf(filename)
		if exclude != conf.Command {
			function(conf)
		}
	}
}

func (d Directory) ForEachAll(function func(conf *Service, action *Action)) {
	d.ForEach(func(conf *Service) {
		conf.ForEach(func(action *Action) {
			function(conf, action)
		})
	}, "searchInAll")
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

func (d Directory) LoadConf(filename string) *Service {
	yfile, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}

	conf := &Service{}
	err1 := yaml.Unmarshal(yfile, conf)
	if err1 != nil {
		log.Fatal(err1)
	}
	conf.Command = path.Base(filename[:len(filename)-5])
	return conf
}
