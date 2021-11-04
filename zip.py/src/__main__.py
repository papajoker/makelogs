#!/usr/bin/env python3
"""
    yaml to object ... ?
    https://stackoverflow.com/questions/49029459/read-yaml-file-and-create-python-objects
"""
# TODO
"""
    yaml interave yaml commands ?
    command: "ls -l %ASK%"
    ask: "Enter directory"
    if %ASK% in command, then prompt question "ask" key ...
"""
import sys
from pathlib import Path
from typing import Generator
import zipfile
import shutil

from io import FileIO
import sys
import os
import re
from pathlib import Path
import subprocess
import locale
from datetime import datetime

from includes import *

from yaml.loader import Loader
try:
    import yaml
except ValueError:
    print("Error: install python-yaml package : sudo pacman -S python-yaml")
    sys.exit(4)


'''
tree:
    ./src/
    ./src/__main__.py
    ./src/yaml/*

python -m zipapp src --python="/usr/bin/env python" --compress --output="makelog.pyz"
./makelog.pyz
'''


configdir = "/tmp/makelogs/yaml"
lang = locale.getdefaultlocale()[0].lower()[3:5]

COLOR_GREEN = '\033[92m'
COLOR_NONE = '\033[0m'
COLOR_BOLD = '\033[1m'
COLOR_GRAY = '\033[38;5;243m'


def extract_resources():
    """
        Extract yaml files in /tmp/
    """

    # TODO set always extract for final.
    if not "-c" in sys.argv and Path(configdir).exists():
        return

    Path(configdir).mkdir(parents=True, exist_ok=True)
    a = Path(__file__).name
    b = Path(sys.argv[0]).name
    if a != b:  # __main__.py != makelogs ?
        """
            we are in .pyz, so we extract from self file zip
        """
        zfile = Path(__file__).parent

        with zipfile.ZipFile(zfile, mode='r') as myzip:
            for file in myzip.namelist():
                if file.endswith(".yaml"):
                    with myzip.open(file) as opened_file:
                        data = opened_file.read().decode()
                        print(
                            f"{COLOR_GRAY}#cp in {Path(configdir).parent}/{file}{COLOR_NONE}")
                        Path(f"{Path(configdir).parent}/{file}").write_text(data)
    else:
        """
            in dev mode, copy from sub directory
        """
        src = Path(__file__).parent / "yaml"
        # for y in Path(src).glob("*.yaml"):
        #    Path(configdir + "/" + y.name).write_text(s)
        shutil.copytree(src, configdir, dirs_exist_ok=True)


######################
# work
######################

def log_to_cloud(log_filename: str):

    def send_coud(name: str, url: str, log_filename: str) -> str:
        cmd = f"cat '{log_filename}' | curl -s -F {url}"
        with subprocess.Popen(
                    [f"{cmd}"],
                    universal_newlines=True,
                    stdout=subprocess.PIPE,
                    shell=True,
                    text=True
                ) as process:
            return process.stdout.read().strip()
        return ""

    out = send_coud("ix.io", "'f:1;read:1=<-' http://ix.io", log_filename)
    if not out:
        out = send_coud("sprunge", "'sprunge=<-' http://sprunge.us?md", log_filename)
        if not out:
            print("Error !", file=sys.stderr)
            exit(1)
    if out:
        print(f'\nUrl to share: "{COLOR_GREEN}{out}{COLOR_NONE}"\n')


def load_yaml(args):
    file_name = f"default.yaml" if not args[1:] else args[1]
    if not Path(file_name).suffix:
        file_name = f"{file_name}.yaml"
    if file_name.startswith("."):
        file_name = f"{Path.cwd()}/{file_name}"
    if not file_name.startswith("/"):
        file_name = f"{configdir}/{file_name}"

    yaml_file = open(file_name, 'r')
    return yaml.load(yaml_file, Loader=yaml.SafeLoader)


def get_action_title(action: dict, lang="en") -> str:
    ret = ""
    try:
        ret = action["title"][lang]
    except KeyError:
        try:
            ret = action["title"]["en"]
        except KeyError:
            pass
    return ret

def get_files_name():
    """ list yaml files """
    lang = locale.getdefaultlocale()[0].lower()[3:5]
    for yaml in Path(configdir).glob('*.yaml'):
        if yaml.name == "search.yaml":
            # this file exists only for search ;)
            continue
        yield yaml


def display_files():
    """ list yaml files and display commands """
    for yaml in get_files_name():
        datas = load_yaml(['', str(yaml)])
        print(
            f"\n{COLOR_GREEN}{yaml.stem:41}{COLOR_NONE}{datas['caption']}{COLOR_NONE}")
        for action in datas['actions']:
            print(
                f"  {action['name']:38}{COLOR_GRAY}{get_action_title(action, lang)}{COLOR_NONE}")


def search_foreach_files(prog: re.Pattern, lan:str = "en") -> Generator:
    """ list yaml files """
    # TODO recherche

    for yaml in Path(configdir).glob("*.yaml"):
        datas = load_yaml(['', str(yaml)])
        for action in datas['actions']:
            title = get_action_title(action, lang)
            cmd = action.get('command', '')
            # print(f"? {COLOR_GRAY}{action['name']} {title} {cmd}{COLOR_NONE}")
            if prog.search(f"{action['name']} {title} {cmd}"):
                yield action


def search_in_files(args: list) -> Generator:
    pattern = ' '.join(args)
    if len(pattern) < 3:
        return 0
    if len(args) > 1:
        pattern = pattern.replace(' ','|')
    if '+' in pattern:
        pattern = pattern.replace('+',".*")
    pattern = re.compile(pattern, flags=re.IGNORECASE)
    # print("::", pattern)
    lang = locale.getdefaultlocale()[0].lower()[3:5]
    results = list(search_foreach_files(pattern, lang))
    max =0
    for i, action in enumerate(results):
        title = get_action_title(action, lang)
        cmd = action.get("command", '')
        print(f"\n{i+1:2} :: {COLOR_GREEN}{action['name']:38}{COLOR_GRAY}{title}{COLOR_NONE}")
        if cmd:
            print(f"{' ':6}{COLOR_GRAY}{cmd}{COLOR_NONE}")
        max = i+1

    choice = input(f"\nrun command: [1..{max if max > 1 else ''}]:  ")
    choice = choice.split(' ')
    print("")
    for i in { int(x, 10) for x in choice if x.isnumeric() and int(x, 10) <= max and int(x) >0}:
        yield results[i-1]


def main(datas, log_filename):

    #if len(datas['actions']) < 1:
    #    return

    if datas.get('sudo', 0) == 1:
        print("\n Command use admin rights, use sudo")
        exit(3)

    results = []

    print(f"### {datas['caption']}\n")
    start_time = datetime.now()
    for action in datas['actions']:
        print("\n")
        print(f"{'-' * 12} {action['name']:^32} {'-' * 12}")
        if title := get_action_title(action, lang):
            print(f"  {title}")
        if cmd := action.get('command', False):
            print(f"  `{cmd}`")

        if requires := action.get('require', False):
            not_run = False
            for require in requires:
                if str(require).startswith("bash:"):
                    require = require[5:].strip()
                    try:
                        subprocess.check_output(require, shell=True)
                    except subprocess.CalledProcessError:
                        print(f'Info: command "{require}" return False')
                        not_run = True
                elif str(require).startswith("/"):
                    if not Path(require).exists():
                        print(f"Info: file {require} not found")
                        not_run = True
                else:
                    # package installed ?
                    require = require.lower()
                    obj = PkgExists({'pkgs': require})
                    obj()
                    if not obj.ok:
                        not_run = True
                    '''
                    ret = [True for p in pamac_db.get_installed_pkgs() if p.get_name() == require]
                    if not ret:
                        print(f"Info: package {require} not installed")
                        not_run = True
                    '''
            if not_run:
                continue

        print("-" * (34+2*12), "\n")

        if cmd := action.get('command', False):
            with subprocess.Popen([f"env TERM=xterm LANG=C {cmd}"], universal_newlines=True, stdout=subprocess.PIPE, shell=True, text=True) as process:
                if out := process.stdout.read():
                    results.append({'action': action, 'output': out})
                    print(out)

        # load and run python class
        # LINK ./includes.py:45
        if cmd := action.get('object', False):
            try:
                class_obj: IncludeJob = globals()[cmd]
                obj = class_obj(action)
                out = obj()
                results.append({'action': action, 'output': out})
                print(out)
            except KeyError:
                print(
                    f'Warning: Object "{action["object"]}" not exits \N{face with head-bandage}', file=sys.stderr)

    if not results:
        exit(12)

    with open(log_filename, "w+") as flog:
        flog.write(f"## {datas['caption']}")
        for item in results:
            flog.write(f"\n\n**{item['action']['name']}**\n")
            if cmd := item['action'].get('command', False):
                flog.write(f"> `{cmd}`\n")
            flog.write(f"\n```\n{item['output']} ```")

    print(f'\nLog saved in: \" {log_filename} \"')

    elapsed_time = datetime.now() - start_time
    print(
        f"\nDuration: {elapsed_time.seconds:2}.{elapsed_time.microseconds:2} second.micro-second")

def usage():
    
    print(f"""\n./{COLOR_GREEN}makelogs{COLOR_NONE} [log]
    -l : List logs available
    -f : Find/run some command
    -s : Send log file in cloud

    -c : refresh Configuration

    [log] : short name listed by command \"-v"
            {COLOR_GRAY}{', '.join([ s.stem for s in get_files_name()])}{COLOR_NONE}
            or local file name with path
    """)
    exit(0)


extract_resources()

if __name__ == "__main__":

    log_filename = f"{Path.cwd()}/logs.md"

    if len(sys.argv) > 1 and sys.argv[1][0] == '-':

        # NOTE "-c" : force rewrite /tmp/datas
        if sys.argv[1].lower() == '-l':
            display_files()
            exit(0)

        if sys.argv[1].lower() == '-f' and len(sys.argv) > 2:
            main({
                "caption":"Search",
                "actions":list(search_in_files(sys.argv[2:]))
                },
                log_filename)
            exit(0)

        if sys.argv[1].lower() == '-s':
            if not Path(log_filename).exists():
                exit(8)
            log_to_cloud(log_filename)
            exit(0)

        usage()
        # exit(127)

    datas = load_yaml(sys.argv)
    os.environ['TERM'] = "xterm"
    
    main(datas, log_filename)

    # TODO send to cloud
    print(f"\ncat '{log_filename}' | curl -F 'sprunge=<-' http://sprunge.us")
