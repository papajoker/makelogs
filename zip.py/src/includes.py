import subprocess
from datetime import datetime
import json

class IncludeJob:
    """ Abstract class for call include object from yaml """

    def __init__(self, action: dict) -> None:
        self.action = action

    def __call__(self):
        raise RuntimeError(f"Abstract Class 🥵")

    def __str__(self) -> str:
        raise RuntimeError(f"Abstract Class 🥵")



class PkgVer(IncludeJob):
    """ list packages with version """

    def __init__(self, action) -> None:
        super().__init__(action)
        self.pkgs = action['pkgs'].lower().split() if action.get('pkgs') else []
        self.logs = ""

    def __call__(self):
        if not self.pkgs:
            return self
        cmd = f"pacman -Qi {' '.join(self.pkgs)} |awk -F':' '/^Name/ {{n=$2}} /^Ver/ {{print n\": \"$2}}'"
        with subprocess.Popen(
                [f"env TERM=xterm LANG=C {cmd}"],
                universal_newlines=True,
                stdout=subprocess.PIPE,
                shell=True,
                text=True
            ) as process:
                self.logs = process.stdout.read()
        return self

    def __str__(self) -> str:
        return self.logs


class Journald(IncludeJob):
    def __init__(self, action) -> None:
        super().__init__(action)
        self.logs = ()
        self.level = action['level'] if action.get('level') else 3

    def _print_item(self, item, old_date) -> tuple[str, str]:
        d = item['DATE'][:16]
        if d == old_date:
            d = ""
        return item['DATE'][:16], f"{d}\n\t({item['PRIORITY']}) [{item['_UID']:>4}] {item['_CMDLINE']}\n\t{item['MESSAGE']}"

    def __str__(self) -> str:
        max = 44
        i = 1
        ret = ""
        old_date = ""
        for item in self.logs:
            i += 1
            old_date, outstr = self._print_item(item, old_date)
            ret = f"{ret}\n{outstr}"
            if i > max:
                break
        return f"\n{ret}\n"

    def __call__(self):
        cmd = f'SYSTEMD_COLORS=0 /usr/bin/journalctl -b0 -p{self.level} --no-pager -o json'
        with subprocess.Popen(cmd, stdout=subprocess.PIPE, bufsize=-1, universal_newlines=True, text=True, shell=True) as proc:
            for line in proc.stdout:
                data = json.loads(line)
                item = {key: value for key, value in data.items() if key in ["PRIORITY", "MESSAGE", "_CMDLINE", "_UID", "__REALTIME_TIMESTAMP"]}
                dt_object = datetime.fromtimestamp(int(item["__REALTIME_TIMESTAMP"][0:10]))
                item['DATE'] = str(dt_object)
                if '_UID' not in item.keys():
                    item['_UID'] = "0"
                if '_CMDLINE' not in item.keys():
                    item['_CMDLINE'] = data["SYSLOG_IDENTIFIER"]
                self.logs = self.logs + (item,)
        if proc.returncode != 0:
            exit(proc.returncode)
        return self
    
class PkgExists(IncludeJob):
    """ package installed ? """
    def __init__(self, action) -> None:
        super().__init__(action)
        self.ok = False
        self.pkgs = action.get('pkgs',"*")

    def __str__(self) -> str:
        return f"\ninstalled: {self.pkgs}\n" if self.ok else ""

    def __call__(self):
        try:
            subprocess.check_output(f'pacman -Qq {self.pkgs} >/dev/null 2>&1', shell=True)
            self.ok = True
        except subprocess.CalledProcessError:
            self.ok = False
        return self
