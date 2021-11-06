#!/usr/bin/env python3
import sys
from console import execute

if __name__ == "__main__":

    if len(sys.argv) > 1 and sys.argv[1] == '--gtk':
        import gtkgui
        exit(0)

    execute()